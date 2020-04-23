package api

import (
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

	"github.com/dstotijn/ct-diag-server/diag"
	"github.com/google/uuid"
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
				Key:       uuid.MustParse("adc69f96-c83f-4c2b-8905-ddf2b6ba8543"),
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
			keyBuf := make([]byte, 16)
			_, err := resp.Body.Read(keyBuf)
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, "Invalid body", http.StatusBadRequest)
				return
			}

			key, err := uuid.FromBytes(keyBuf)
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
