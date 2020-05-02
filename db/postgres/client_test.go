package postgres

import (
	"context"
	"crypto/rand"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/dstotijn/ct-diag-server/diag"
)

var client *Client

func TestMain(m *testing.M) {
	var err error

	client, err = New(os.Getenv("POSTGRES_DSN"))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	for i := 0; i < 10; i++ {
		err = client.Ping()
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(m.Run())
}

func TestStoreDiagnosisKeys(t *testing.T) {
	ctx := context.Background()
	key := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	tests := []struct {
		name        string
		diagKeys    []diag.DiagnosisKey
		expDiagKeys []diag.DiagnosisKey
		expError    error
	}{
		{
			name:     "empty input array",
			diagKeys: nil,
			expError: diag.ErrNilDiagKeys,
		},
		{
			name: "valid diagnosis keyset",
			diagKeys: []diag.DiagnosisKey{
				{
					TemporaryExposureKey: key,
					ENIntervalNumber:     uint32(42),
				},
			},
			expDiagKeys: []diag.DiagnosisKey{
				{
					TemporaryExposureKey: key,
					ENIntervalNumber:     uint32(42),
				},
			},
			expError: nil,
		},
		{
			name: "duplicate diagnosis keyset",
			diagKeys: []diag.DiagnosisKey{
				{
					TemporaryExposureKey: key,
					ENIntervalNumber:     uint32(42),
				},
				{
					TemporaryExposureKey: key,
					ENIntervalNumber:     uint32(42),
				},
			},
			expDiagKeys: []diag.DiagnosisKey{
				{
					TemporaryExposureKey: key,
					ENIntervalNumber:     uint32(42),
				},
			},
			expError: nil,
		},
	}

	for _, tt := range tests {
		_, err := client.db.ExecContext(ctx, "TRUNCATE diagnosis_keys")
		if err != nil {
			t.Fatal(err)
		}

		t.Run(tt.name, func(t *testing.T) {
			err := client.StoreDiagnosisKeys(ctx, tt.diagKeys, time.Now())
			if err != tt.expError {
				t.Fatalf("expected: %v, got: %v", tt.expError, err)
			}

			var diagKeys []diag.DiagnosisKey

			rows, err := client.db.QueryContext(ctx, "SELECT key, interval_number FROM diagnosis_keys")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()

			for rows.Next() {
				var diagKey diag.DiagnosisKey
				key := make([]byte, 0, 16)
				err := rows.Scan(&key, &diagKey.ENIntervalNumber)
				if err != nil {
					t.Fatal(err)
				}
				copy(diagKey.TemporaryExposureKey[:], key)
				diagKeys = append(diagKeys, diagKey)
			}
			rows.Close()

			err = rows.Err()
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(diagKeys, tt.expDiagKeys) {
				t.Errorf("expected: %#v, got: %#v", tt.expDiagKeys, diagKeys)
			}
		})
	}
}

func TestFindAllDiagnosisKeys(t *testing.T) {
	ctx := context.Background()
	key := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	_, err := client.db.ExecContext(ctx, "TRUNCATE diagnosis_keys")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		diagKeys    []diag.DiagnosisKey
		expDiagKeys []diag.DiagnosisKey
		expError    error
	}{
		{
			name:        "no diagnosis keys in database",
			diagKeys:    nil,
			expDiagKeys: nil,
			expError:    nil,
		},
		{
			name: "diagnosis keys in database",
			diagKeys: []diag.DiagnosisKey{
				{
					TemporaryExposureKey: key,
					ENIntervalNumber:     uint32(42),
				},
			},
			expDiagKeys: []diag.DiagnosisKey{
				{
					TemporaryExposureKey: key,
					ENIntervalNumber:     uint32(42),
				},
			},
			expError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := client.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()

			stmt, err := tx.PrepareContext(ctx, "INSERT INTO diagnosis_keys (key, interval_number, created_at) VALUES ($1, $2, $3)")
			if err != nil {
				t.Fatal(err)
			}
			defer stmt.Close()

			createdAt := time.Now()

			for _, diagKey := range tt.diagKeys {
				_, err = stmt.ExecContext(ctx, diagKey.TemporaryExposureKey[:], diagKey.ENIntervalNumber, createdAt)
				if err != nil {
					t.Fatal(err)
				}
			}

			err = tx.Commit()
			if err != nil {
				t.Fatal(err)
			}

			diagKeys, err := client.FindAllDiagnosisKeys(ctx)
			if err != tt.expError {
				t.Fatalf("expected: %v, got: %v", tt.expError, err)
			}

			if !reflect.DeepEqual(diagKeys, tt.expDiagKeys) {
				t.Errorf("expected: %#v, got: %#v", tt.expDiagKeys, diagKeys)
			}
		})
	}
}

func TestLastModified(t *testing.T) {
	ctx := context.Background()

	_, err := client.db.ExecContext(ctx, "TRUNCATE diagnosis_keys")
	if err != nil {
		t.Fatal(err)
	}

	randomTEK := func() (buf [16]byte) {
		if _, err := rand.Read(buf[:]); err != nil {
			t.Fatal(err)
		}
		return
	}

	type storeReq struct {
		diagKey      diag.DiagnosisKey
		lastModified time.Time
	}

	tests := []struct {
		name            string
		storeReq        []storeReq
		expLastModified time.Time
		expError        error
	}{
		{
			name:            "no diagnosis keys in database",
			storeReq:        nil,
			expLastModified: time.Time{},
			expError:        diag.ErrNilDiagKeys,
		},
		{
			name: "diagnosis keys in database",
			storeReq: []storeReq{
				{
					diagKey: diag.DiagnosisKey{
						TemporaryExposureKey: randomTEK(),
						ENIntervalNumber:     uint32(42),
					},
					lastModified: time.Unix(42, 0),
				},
				{
					diagKey: diag.DiagnosisKey{
						TemporaryExposureKey: randomTEK(),
						ENIntervalNumber:     uint32(42),
					},
					lastModified: time.Unix(43, 0),
				},
			},
			expLastModified: time.Unix(43, 0),
			expError:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := client.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()

			stmt, err := tx.PrepareContext(ctx, "INSERT INTO diagnosis_keys (key, interval_number, created_at) VALUES ($1, $2, $3)")
			if err != nil {
				t.Fatal(err)
			}
			defer stmt.Close()

			for _, storeReq := range tt.storeReq {
				_, err = stmt.ExecContext(ctx, storeReq.diagKey.TemporaryExposureKey[:], storeReq.diagKey.ENIntervalNumber, storeReq.lastModified)
				if err != nil {
					t.Fatal(err)
				}
			}

			err = tx.Commit()
			if err != nil {
				t.Fatal(err)
			}

			lastModified, err := client.LastModified(ctx)
			if err != tt.expError {
				t.Fatalf("expected: %v, got: %v", tt.expError, err)
			}

			if !lastModified.Equal(tt.expLastModified) {
				t.Errorf("expected: %v, got: %v", tt.expLastModified, lastModified)
			}
		})
	}
}
