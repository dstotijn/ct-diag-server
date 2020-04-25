package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/dstotijn/ct-diag-server/diag"
)

type testRepository struct {
	storeDiagnosisKeysFn   func(context.Context, []diag.DiagnosisKey) error
	findAllDiagnosisKeysFn func(context.Context) ([]diag.DiagnosisKey, error)
}

func (ts testRepository) StoreDiagnosisKeys(ctx context.Context, diagKeys []diag.DiagnosisKey) error {
	return ts.storeDiagnosisKeysFn(ctx, diagKeys)
}

func (ts testRepository) FindAllDiagnosisKeys(ctx context.Context) ([]diag.DiagnosisKey, error) {
	return ts.findAllDiagnosisKeysFn(ctx)
}

func TestHealth(t *testing.T) {
	handler := NewHandler(nil)
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
		repo := testRepository{
			findAllDiagnosisKeysFn: func(_ context.Context) ([]diag.DiagnosisKey, error) {
				return nil, nil
			},
		}
		handler := NewHandler(repo)
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
				Key:       [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				DayNumber: uint16(42),
			},
		}
		repo := testRepository{
			findAllDiagnosisKeysFn: func(_ context.Context) ([]diag.DiagnosisKey, error) {
				return expDiagKeys, nil
			},
		}

		handler := NewHandler(repo)
		req := httptest.NewRequest("GET", "http://example.com/diagnosis-keys", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		resp := w.Result()

		expStatusCode := 200
		if got := resp.StatusCode; got != expStatusCode {
			t.Errorf("expected: %v, got: %v", expStatusCode, got)
		}

		expContentLength := strconv.Itoa(len(expDiagKeys) * 18)
		if got := resp.Header.Get("Content-Length"); got != expContentLength {
			t.Fatalf("expected: %v, got: %v", expContentLength, got)
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

			var dayNumber uint16
			err = binary.Read(resp.Body, binary.BigEndian, &dayNumber)
			if err == io.EOF {
				t.Fatal(err)
			}
			if err != nil {
				t.Fatal(err)
			}

			got = append(got, diag.DiagnosisKey{Key: key, DayNumber: dayNumber})
		}

		if !reflect.DeepEqual(got, expDiagKeys) {
			t.Errorf("expected: %#v, got: %#v", expDiagKeys, got)
		}
	})

	t.Run("diag.Service returns error", func(t *testing.T) {
		repo := testRepository{
			findAllDiagnosisKeysFn: func(_ context.Context) ([]diag.DiagnosisKey, error) {
				return nil, errors.New("foobar")
			},
		}
		handler := NewHandler(repo)
		req := httptest.NewRequest("GET", "http://example.com/diagnosis-keys", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		resp := w.Result()

		expStatusCode := 500
		if got := resp.StatusCode; got != expStatusCode {
			t.Errorf("expected: %v, got: %v", expStatusCode, got)
		}

		expBody := "Internal Server Error"
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		if got := strings.TrimSpace(string(body)); got != expBody {
			t.Errorf("expected: %v, got: `%s`", expBody, got)
		}
	})
}

func TestPostDiagnosisKeys(t *testing.T) {
	t.Run("missing post body", func(t *testing.T) {
		handler := NewHandler(nil)
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
		handler := NewHandler(nil)
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
			Key:       [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			DayNumber: uint16(42),
		}

		buf := &bytes.Buffer{}
		for i := 0; i < diag.MaxUploadBatchSize+1; i++ {
			_, err := buf.Write(diagKey.Key[:])
			if err != nil {
				panic(err)
			}
			err = binary.Write(buf, binary.BigEndian, diagKey.DayNumber)
			if err != nil {
				panic(err)
			}
		}
		handler := NewHandler(nil)
		req := httptest.NewRequest("POST", "http://example.com/diagnosis-keys", buf)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		resp := w.Result()

		expStatusCode := 413
		if got := resp.StatusCode; got != expStatusCode {
			t.Errorf("expected: %v, got: %v", expStatusCode, got)
		}

		expBody := "Request Entity Too Large"
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
				Key:       [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				DayNumber: uint16(42),
			},
		}

		validBody := func() *bytes.Buffer {
			buf := &bytes.Buffer{}
			for _, expDiagKey := range expDiagKeys {
				_, err := buf.Write(expDiagKey.Key[:])
				if err != nil {
					panic(err)
				}
				err = binary.Write(buf, binary.BigEndian, expDiagKey.DayNumber)
				if err != nil {
					panic(err)
				}
			}

			return buf
		}

		t.Run("diag.Service returns nil error", func(t *testing.T) {
			var storedDiagKeys []diag.DiagnosisKey
			repo := testRepository{
				storeDiagnosisKeysFn: func(_ context.Context, diagKeys []diag.DiagnosisKey) error {
					storedDiagKeys = diagKeys
					return nil
				},
			}
			handler := NewHandler(repo)

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
			repo := testRepository{
				storeDiagnosisKeysFn: func(_ context.Context, diagKeys []diag.DiagnosisKey) error {
					return errors.New("foobar")
				},
			}
			handler := NewHandler(repo)

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
	handler := NewHandler(nil)
	req := httptest.NewRequest("PATCH", "http://example.com/diagnosis-keys", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	resp := w.Result()

	expStatusCode := 405
	if got := resp.StatusCode; got != expStatusCode {
		t.Errorf("expected: %v, got: %v", expStatusCode, got)
	}
}
