package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dstotijn/ct-diag-server/diag"

	"go.uber.org/zap"
)

type testRepository struct {
	storeDiagnosisKeysFn   func(context.Context, []diag.DiagnosisKey, time.Time) error
	findAllDiagnosisKeysFn func(context.Context) ([]diag.DiagnosisKey, error)
	lastModifiedFn         func(context.Context) (time.Time, error)
}

func (ts testRepository) StoreDiagnosisKeys(ctx context.Context, diagKeys []diag.DiagnosisKey, createdAt time.Time) error {
	return ts.storeDiagnosisKeysFn(ctx, diagKeys, createdAt)
}

func (ts testRepository) FindAllDiagnosisKeys(ctx context.Context) ([]diag.DiagnosisKey, error) {
	return ts.findAllDiagnosisKeysFn(ctx)
}

func (ts testRepository) LastModified(ctx context.Context) (time.Time, error) {
	return ts.lastModifiedFn(ctx)
}

var noopRepo = testRepository{
	storeDiagnosisKeysFn:   func(_ context.Context, _ []diag.DiagnosisKey, _ time.Time) error { return nil },
	findAllDiagnosisKeysFn: func(_ context.Context) ([]diag.DiagnosisKey, error) { return nil, nil },
	lastModifiedFn:         func(_ context.Context) (time.Time, error) { return time.Time{}, nil },
}

func newTestHandler(t *testing.T, cfg *diag.Config) http.Handler {
	if cfg == nil {
		cfg = &diag.Config{Repository: noopRepo}
	}

	logger := zap.NewNop()
	if cfg.Logger == nil {
		cfg.Logger = logger
	}

	handler, err := NewHandler(context.Background(), *cfg, logger)
	if err != nil {
		t.Fatal(err)
	}

	return handler
}

func TestHealth(t *testing.T) {
	handler := newTestHandler(t, nil)

	req := httptest.NewRequest("GET", "http://example.com/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	resp := w.Result()

	expStatusCode := 200
	if got := resp.StatusCode; got != expStatusCode {
		t.Errorf("expected: %v, got: %v", expStatusCode, got)
	}

	expBody := "OK"
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if got := strings.TrimSpace(string(body)); got != expBody {
		t.Errorf("expected: %v, got: `%s`", expBody, got)
	}
}

func TestListDiagnosisKeys(t *testing.T) {
	t.Run("no diagnosis keys found", func(t *testing.T) {
		handler := newTestHandler(t, nil)
		req := httptest.NewRequest("GET", "http://example.com/diagnosis-keys", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		resp := w.Result()

		expStatusCode := 200
		if got := resp.StatusCode; got != expStatusCode {
			t.Errorf("expected: %v, got: %v", expStatusCode, got)
		}

		expContentLength := "0"
		if got := resp.Header.Get("Content-Length"); got != expContentLength {
			t.Errorf("expected: %v, got: %v", expContentLength, got)
		}
	})

	t.Run("diagnosis keys found", func(t *testing.T) {
		expDiagKeys := []diag.DiagnosisKey{
			{
				TemporaryExposureKey: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				ENIntervalNumber:     uint32(42),
			},
		}
		expLastModified := time.Date(2020, time.May, 2, 23, 30, 0, 0, time.UTC)
		cfg := &diag.Config{
			Repository: testRepository{
				findAllDiagnosisKeysFn: func(_ context.Context) ([]diag.DiagnosisKey, error) {
					return expDiagKeys, nil
				},
				lastModifiedFn: func(_ context.Context) (time.Time, error) { return expLastModified, nil },
			},
		}

		handler := newTestHandler(t, cfg)
		req := httptest.NewRequest("GET", "http://example.com/diagnosis-keys", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		resp := w.Result()

		expStatusCode := 200
		if got := resp.StatusCode; got != expStatusCode {
			t.Errorf("expected: %v, got: %v", expStatusCode, got)
		}

		expContentLength := strconv.Itoa(len(expDiagKeys) * 20)
		if got := resp.Header.Get("Content-Length"); got != expContentLength {
			t.Fatalf("expected: %v, got: %v", expContentLength, got)
		}

		if got := resp.Header.Get("Last-Modified"); got != expLastModified.Format(http.TimeFormat) {
			t.Fatalf("expected: %v, got: %v", expLastModified.Format(http.TimeFormat), got)
		}

		var got []diag.DiagnosisKey

		for {
			var key [16]byte
			_, err := io.ReadFull(resp.Body, key[:])
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}

			var enin uint32
			err = binary.Read(resp.Body, binary.BigEndian, &enin)
			if err == io.EOF {
				t.Fatal(err)
			}
			if err != nil {
				t.Fatal(err)
			}

			got = append(got, diag.DiagnosisKey{TemporaryExposureKey: key, ENIntervalNumber: enin})
		}

		if !reflect.DeepEqual(got, expDiagKeys) {
			t.Errorf("expected: %#v, got: %#v", expDiagKeys, got)
		}
	})

	t.Run("with `since` query parameter", func(t *testing.T) {
		tests := []struct {
			name        string
			diagKeys    []diag.DiagnosisKey
			expDiagKeys []diag.DiagnosisKey
			since       string
		}{
			{
				name:        "no diagnosis keys in database",
				diagKeys:    nil,
				expDiagKeys: nil,
			},
			{
				name: "since date on oldest created_at day in database",
				diagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 02, 13, 37, 0, 0, time.UTC),
					},
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 03, 13, 37, 0, 0, time.UTC),
					},
				},
				expDiagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
						ENIntervalNumber:     uint32(42),
					},
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						ENIntervalNumber:     uint32(42),
					},
				},
				since: "2020-05-02",
			},
			{
				name: "since date on latest created_at day in database",
				diagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 02, 13, 37, 0, 0, time.UTC),
					},
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 03, 13, 37, 0, 0, time.UTC),
					},
				},
				expDiagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						ENIntervalNumber:     uint32(42),
					},
				},
				since: "2020-05-03",
			},
			{
				name: "since date older than oldest created_at day in database",
				diagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 02, 13, 37, 0, 0, time.UTC),
					},
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 03, 13, 37, 0, 0, time.UTC),
					},
				},
				expDiagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
						ENIntervalNumber:     uint32(42),
					},
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						ENIntervalNumber:     uint32(42),
					},
				},
				since: "2020-05-01",
			},
			{
				name: "since date later than newest created_at day in database",
				diagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 02, 13, 37, 0, 0, time.UTC),
					},
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 03, 13, 37, 0, 0, time.UTC),
					},
				},
				expDiagKeys: nil,
				since:       "2020-05-04",
			},
			{
				name: "since date between oldest and newest created_at day in database",
				diagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 02, 13, 37, 0, 0, time.UTC),
					},
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 03, 13, 37, 0, 0, time.UTC),
					},
					{
						TemporaryExposureKey: [16]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 04, 13, 37, 0, 0, time.UTC),
					},
				},
				expDiagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						ENIntervalNumber:     uint32(42),
					},
					{
						TemporaryExposureKey: [16]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3},
						ENIntervalNumber:     uint32(42),
					},
				},
				since: "2020-05-03",
			},
			{
				name: "since date between oldest and newest, but in between day offsets",
				diagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 02, 13, 37, 0, 0, time.UTC),
					},
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						ENIntervalNumber:     uint32(42),
						UploadedAt:           time.Date(2020, 05, 04, 13, 37, 0, 0, time.UTC),
					},
				},
				expDiagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
						ENIntervalNumber:     uint32(42),
					},
				},
				since: "2020-05-03",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := &diag.Config{
					Repository: testRepository{
						findAllDiagnosisKeysFn: func(_ context.Context) ([]diag.DiagnosisKey, error) {
							return tt.diagKeys, nil
						},
						lastModifiedFn: noopRepo.lastModifiedFn,
					},
				}

				handler := newTestHandler(t, cfg)
				req := httptest.NewRequest("GET", "http://example.com/diagnosis-keys", nil)
				qp := req.URL.Query()
				qp.Add("since", tt.since)
				req.URL.RawQuery = qp.Encode()
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)
				resp := w.Result()

				expStatusCode := 200
				if got := resp.StatusCode; got != expStatusCode {
					body, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						t.Fatal(err)
					}
					t.Errorf("expected: %v, got: %v (body: %s)", expStatusCode, got, strings.TrimSpace(string(body)))
				}

				var got []diag.DiagnosisKey

				for {
					var key [16]byte
					_, err := io.ReadFull(resp.Body, key[:])
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}

					var enin uint32
					err = binary.Read(resp.Body, binary.BigEndian, &enin)
					if err == io.EOF {
						t.Fatal(err)
					}
					if err != nil {
						t.Fatal(err)
					}

					got = append(got, diag.DiagnosisKey{TemporaryExposureKey: key, ENIntervalNumber: enin})
				}

				if !reflect.DeepEqual(got, tt.expDiagKeys) {
					t.Errorf("expected: %#v, got: %#v", tt.expDiagKeys, got)
				}
			})
		}
	})
}

func TestPostDiagnosisKeys(t *testing.T) {
	t.Run("missing post body", func(t *testing.T) {
		handler := newTestHandler(t, nil)
		req := httptest.NewRequest("POST", "http://example.com/diagnosis-keys", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		resp := w.Result()

		expStatusCode := 400
		if got := resp.StatusCode; got != expStatusCode {
			t.Errorf("expected: %v, got: %v", expStatusCode, got)
		}

		expBody := "Invalid body: unexpected EOF"
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		if got := strings.TrimSpace(string(body)); got != expBody {
			t.Errorf("expected: %v, got: `%s`", expBody, got)
		}
	})

	t.Run("incomplete diagnosis key", func(t *testing.T) {
		handler := newTestHandler(t, nil)
		body := bytes.NewReader([]byte{0x00})
		req := httptest.NewRequest("POST", "http://example.com/diagnosis-keys", body)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		resp := w.Result()

		expStatusCode := 400
		if got := resp.StatusCode; got != expStatusCode {
			t.Errorf("expected: %v, got: %v", expStatusCode, got)
		}

		expBody := "Invalid body: unexpected EOF"
		resBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		if got := strings.TrimSpace(string(resBody)); got != expBody {
			t.Errorf("expected: %v, got: `%s`", expBody, got)
		}
	})

	t.Run("too many diagnosis keys", func(t *testing.T) {
		diagKey := diag.DiagnosisKey{
			TemporaryExposureKey: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			ENIntervalNumber:     uint32(42),
		}

		cfg := &diag.Config{
			Repository:         noopRepo,
			MaxUploadBatchSize: 7,
		}
		handler := newTestHandler(t, cfg)

		buf := &bytes.Buffer{}
		for i := 0; i < int(cfg.MaxUploadBatchSize)+1; i++ {
			_, err := buf.Write(diagKey.TemporaryExposureKey[:])
			if err != nil {
				panic(err)
			}
			err = binary.Write(buf, binary.BigEndian, diagKey.ENIntervalNumber)
			if err != nil {
				panic(err)
			}
		}

		req := httptest.NewRequest("POST", "http://example.com/diagnosis-keys", buf)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		resp := w.Result()

		expStatusCode := 400
		if got := resp.StatusCode; got != expStatusCode {
			t.Errorf("expected: %v, got: %v", expStatusCode, got)
		}

		expBody := "Invalid body: http: request body too large"
		resBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		if got := strings.TrimSpace(string(resBody)); got != expBody {
			t.Fatalf("expected: %v, got: `%s`", expBody, got)
		}
	})

	t.Run("valid diagnosis key", func(t *testing.T) {
		expDiagKeys := []diag.DiagnosisKey{
			{
				TemporaryExposureKey: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				ENIntervalNumber:     uint32(42),
			},
		}

		validBody := func() *bytes.Buffer {
			buf := &bytes.Buffer{}
			for _, expDiagKey := range expDiagKeys {
				_, err := buf.Write(expDiagKey.TemporaryExposureKey[:])
				if err != nil {
					panic(err)
				}
				err = binary.Write(buf, binary.BigEndian, expDiagKey.ENIntervalNumber)
				if err != nil {
					panic(err)
				}
			}

			return buf
		}

		t.Run("diag.Service returns nil error", func(t *testing.T) {
			var storedDiagKeys []diag.DiagnosisKey
			cfg := &diag.Config{
				Repository: testRepository{
					storeDiagnosisKeysFn: func(_ context.Context, diagKeys []diag.DiagnosisKey, _ time.Time) error {
						storedDiagKeys = diagKeys
						return nil
					},
					lastModifiedFn:         noopRepo.lastModifiedFn,
					findAllDiagnosisKeysFn: noopRepo.findAllDiagnosisKeysFn,
				},
			}
			handler := newTestHandler(t, cfg)

			req := httptest.NewRequest("POST", "http://example.com/diagnosis-keys", validBody())
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)
			resp := w.Result()

			expStatusCode := 200
			if got := resp.StatusCode; got != expStatusCode {
				t.Errorf("expected: %v, got: %v", expStatusCode, got)
			}

			expBody := "OK"
			resBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}

			if got := strings.TrimSpace(string(resBody)); got != expBody {
				t.Fatalf("expected: %v, got: `%s`", expBody, got)
			}

			if !reflect.DeepEqual(storedDiagKeys, expDiagKeys) {
				t.Errorf("expected: %#v, got: %#v", expDiagKeys, storedDiagKeys)
			}
		})

		t.Run("diag.Service returns unexpected error", func(t *testing.T) {
			cfg := &diag.Config{
				Repository: testRepository{
					findAllDiagnosisKeysFn: noopRepo.findAllDiagnosisKeysFn,
					storeDiagnosisKeysFn: func(_ context.Context, diagKeys []diag.DiagnosisKey, _ time.Time) error {
						return errors.New("foobar")
					},
					lastModifiedFn: noopRepo.lastModifiedFn,
				}}
			handler := newTestHandler(t, cfg)

			req := httptest.NewRequest("POST", "http://example.com/diagnosis-keys", validBody())
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)
			resp := w.Result()

			expStatusCode := 500
			if got := resp.StatusCode; got != expStatusCode {
				t.Errorf("expected: %v, got: %v", expStatusCode, got)
			}

			expBody := "Internal Server Error"
			resBody, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}

			if got := strings.TrimSpace(string(resBody)); got != expBody {
				t.Fatalf("expected: %v, got: `%s`", expBody, got)
			}
		})
	})
}

func TestUnsupportedMethod(t *testing.T) {
	handler := newTestHandler(t, nil)
	req := httptest.NewRequest("PATCH", "http://example.com/diagnosis-keys", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	resp := w.Result()

	expStatusCode := 405
	if got := resp.StatusCode; got != expStatusCode {
		t.Errorf("expected: %v, got: %v", expStatusCode, got)
	}
}
