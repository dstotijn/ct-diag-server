package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
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
	findAllDiagnosisKeysFn func(context.Context) ([]byte, error)
	lastModifiedFn         func(context.Context) (time.Time, error)
}

func (ts testRepository) StoreDiagnosisKeys(ctx context.Context, diagKeys []diag.DiagnosisKey, createdAt time.Time) error {
	return ts.storeDiagnosisKeysFn(ctx, diagKeys, createdAt)
}

func (ts testRepository) FindAllDiagnosisKeys(ctx context.Context) ([]byte, error) {
	return ts.findAllDiagnosisKeysFn(ctx)
}

func (ts testRepository) LastModified(ctx context.Context) (time.Time, error) {
	return ts.lastModifiedFn(ctx)
}

var noopRepo = testRepository{
	storeDiagnosisKeysFn:   func(_ context.Context, _ []diag.DiagnosisKey, _ time.Time) error { return nil },
	findAllDiagnosisKeysFn: func(_ context.Context) ([]byte, error) { return nil, nil },
	lastModifiedFn:         func(_ context.Context) (time.Time, error) { return time.Time{}, nil },
}

func newTestHandler(t testing.TB, cfg *diag.Config) http.Handler {
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

func TestExposureConfig(t *testing.T) {
	exp := diag.ExposureConfig{
		MinimumRiskScore:                 0,
		AttenuationLevelValues:           []int{1, 2, 3, 4, 5, 6, 7, 8},
		AttenuationWeight:                50,
		DaysSinceLastExposureLevelValues: []int{1, 2, 3, 4, 5, 6, 7, 8},
		DaysSinceLastExposureWeight:      50,
		DurationLevelValues:              []int{1, 2, 3, 4, 5, 6, 7, 8},
		DurationWeight:                   50,
		TransmissionRiskLevelValues:      []int{1, 2, 3, 4, 5, 6, 7, 8},
		TransmissionRiskWeight:           50,
	}

	handler := newTestHandler(t, &diag.Config{
		Repository:     noopRepo,
		ExposureConfig: exp,
	})

	req := httptest.NewRequest("GET", "http://example.com/exposure-config", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	resp := w.Result()

	expStatusCode := 200
	if got := resp.StatusCode; got != expStatusCode {
		t.Errorf("expected: %v, got: %v", expStatusCode, got)
	}

	expContentType := "application/json"
	if got := resp.Header.Get("Content-Type"); got != expContentType {
		t.Errorf("expected: %v, got: %v", expContentType, got)
	}

	var got diag.ExposureConfig
	err := json.NewDecoder(resp.Body).Decode(&got)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(exp, got) {
		t.Errorf("expected: %v, got: `%v`", exp, got)
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
				TemporaryExposureKey:  [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				RollingStartNumber:    uint32(42),
				TransmissionRiskLevel: 50,
			},
		}
		expLastModified := time.Date(2020, time.May, 2, 23, 30, 0, 0, time.UTC)
		cfg := &diag.Config{
			Repository: testRepository{
				findAllDiagnosisKeysFn: func(_ context.Context) ([]byte, error) {
					buf := &bytes.Buffer{}
					diag.WriteDiagnosisKeys(buf, expDiagKeys...)
					return buf.Bytes(), nil
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

		expContentLength := strconv.Itoa(len(expDiagKeys) * diag.DiagnosisKeySize)
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

			var rollingStartNumber uint32
			err = binary.Read(resp.Body, binary.BigEndian, &rollingStartNumber)
			if err != nil {
				t.Fatal(err)
			}

			buf := make([]byte, 1)
			_, err = resp.Body.Read(buf)
			if err != nil {
				t.Fatal(err)
			}

			got = append(got, diag.DiagnosisKey{
				TemporaryExposureKey:  key,
				RollingStartNumber:    rollingStartNumber,
				TransmissionRiskLevel: buf[0],
			})
		}

		if !reflect.DeepEqual(got, expDiagKeys) {
			t.Errorf("expected: %#v, got: %#v", expDiagKeys, got)
		}
	})

	t.Run("with `after` query parameter", func(t *testing.T) {
		tests := []struct {
			name          string
			diagKeys      []diag.DiagnosisKey
			after         string
			expStatusCode int
			expBody       string
			expDiagKeys   []diag.DiagnosisKey
		}{
			{
				name:          "invalid query parameter",
				diagKeys:      nil,
				after:         "foobar",
				expStatusCode: 400,
				expDiagKeys:   nil,
				expBody:       "Invalid `after` query parameter, must be the hexadecimal encoding of a 16 byte key.",
			},
			{
				name:          "no diagnosis keys in database",
				diagKeys:      nil,
				after:         "a7752b99be501c9c9e893b213ad82842",
				expStatusCode: 200,
				expDiagKeys:   nil,
			},
			{
				name:  "after is earliest key in database",
				after: "01010101010101010101010101010101",
				diagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
					},
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
					},
				},
				expStatusCode: 200,
				expDiagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
					},
				},
			},
			{
				name:  "after is latest key in database",
				after: "02020202020202020202020202020202",
				diagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
					},
					{
						TemporaryExposureKey: [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
					},
				},
				expStatusCode: 200,
				expDiagKeys:   nil,
			},
			{
				name:  "after key not found",
				after: "a7752b99be501c9c9e893b213ad82842",
				diagKeys: []diag.DiagnosisKey{
					{
						TemporaryExposureKey: [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
					},
				},
				expStatusCode: 200,
				expDiagKeys:   nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := &diag.Config{
					Repository: testRepository{
						findAllDiagnosisKeysFn: func(_ context.Context) ([]byte, error) {
							buf := &bytes.Buffer{}
							diag.WriteDiagnosisKeys(buf, tt.diagKeys...)
							return buf.Bytes(), nil
						},
						lastModifiedFn: noopRepo.lastModifiedFn,
					},
				}

				handler := newTestHandler(t, cfg)
				req := httptest.NewRequest("GET", "http://example.com/diagnosis-keys", nil)
				qp := req.URL.Query()
				qp.Add("after", tt.after)
				req.URL.RawQuery = qp.Encode()
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)
				resp := w.Result()

				if got := resp.StatusCode; got != tt.expStatusCode {
					t.Errorf("expected: %v, got: %v", tt.expStatusCode, got)
				}

				if tt.expBody != "" {
					body, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						t.Fatal(err)
					}
					if got := strings.TrimSpace(string(body)); got != tt.expBody {
						t.Fatalf("expected: %v, got: `%s`", tt.expBody, got)
					}
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

					var rollingStartNumber uint32
					err = binary.Read(resp.Body, binary.BigEndian, &rollingStartNumber)
					if err != nil {
						t.Fatal(err)
					}
					buf := make([]byte, 1)
					_, err = resp.Body.Read(buf)

					got = append(got, diag.DiagnosisKey{
						TemporaryExposureKey:  key,
						RollingStartNumber:    rollingStartNumber,
						TransmissionRiskLevel: buf[0],
					})
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
			RollingStartNumber:   uint32(42),
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
			err = binary.Write(buf, binary.BigEndian, diagKey.RollingStartNumber)
			if err != nil {
				panic(err)
			}
			err = binary.Write(buf, binary.BigEndian, diagKey.TransmissionRiskLevel)
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
				RollingStartNumber:   uint32(42),
			},
		}

		validBody := func() *bytes.Buffer {
			buf := &bytes.Buffer{}
			for _, expDiagKey := range expDiagKeys {
				_, err := buf.Write(expDiagKey.TemporaryExposureKey[:])
				if err != nil {
					panic(err)
				}
				err = binary.Write(buf, binary.BigEndian, expDiagKey.RollingStartNumber)
				if err != nil {
					panic(err)
				}
				err = binary.Write(buf, binary.BigEndian, expDiagKey.TransmissionRiskLevel)
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

var benchResp *http.Response

func runListDiagnosisKeysBenchmark(b *testing.B, keyCount int) func(b *testing.B) {
	return func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
			}
		})
	}
}

func BenchmarkListDiagnosisKeys(b *testing.B) {
	benchmarks := []struct {
		name         string
		diagKeyCount int
	}{
		{"0", 0},
		{"1000", 1000},
		{"10000", 10000},
		{"100000", 100000},
		{"1000000", 1000000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			cache := &diag.MemoryCache{}
			handler := newTestHandler(b, &diag.Config{
				Repository: noopRepo,
				Cache:      cache,
			})

			// Manually set in-memory cache with random data.
			buf := make([]byte, bm.diagKeyCount*diag.DiagnosisKeySize)
			if _, err := rand.Read(buf); err != nil {
				b.Fatal(err)
			}
			cache.Set(buf, time.Now())

			req := httptest.NewRequest("GET", "http://example.com/diagnosis-keys", nil)
			w := httptest.NewRecorder()

			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				handler.ServeHTTP(w, req)
				benchResp = w.Result()
			}
		})
	}
}
