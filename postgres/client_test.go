package postgres

import (
	"context"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/dstotijn/ct-diag-server/diag"
	"github.com/google/uuid"
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
					Key:       uuid.MustParse("adc69f96-c83f-4c2b-8905-ddf2b6ba8543"),
					DayNumber: uint16(42),
				},
			},
			expDiagKeys: []diag.DiagnosisKey{
				{
					Key:       uuid.MustParse("adc69f96-c83f-4c2b-8905-ddf2b6ba8543"),
					DayNumber: uint16(42),
				},
			},
			expError: nil,
		},
		{
			name: "duplicate diagnosis keyset",
			diagKeys: []diag.DiagnosisKey{
				{
					Key:       uuid.MustParse("adc69f96-c83f-4c2b-8905-ddf2b6ba8543"),
					DayNumber: uint16(42),
				},
				{
					Key:       uuid.MustParse("adc69f96-c83f-4c2b-8905-ddf2b6ba8543"),
					DayNumber: uint16(42),
				},
			},
			expDiagKeys: []diag.DiagnosisKey{
				{
					Key:       uuid.MustParse("adc69f96-c83f-4c2b-8905-ddf2b6ba8543"),
					DayNumber: uint16(42),
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
			err := client.StoreDiagnosisKeys(ctx, tt.diagKeys)
			if err != tt.expError {
				t.Fatalf("expected: %v, got: %v", tt.expError, err)
			}

			var diagKeys []diag.DiagnosisKey

			rows, err := client.db.QueryContext(ctx, "SELECT key, day_number FROM diagnosis_keys")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()

			for rows.Next() {
				var diagKey diag.DiagnosisKey
				err := rows.Scan(&diagKey.Key, &diagKey.DayNumber)
				if err != nil {
					t.Fatal(err)
				}
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
					Key:       uuid.MustParse("adc69f96-c83f-4c2b-8905-ddf2b6ba8543"),
					DayNumber: uint16(42),
				},
			},
			expDiagKeys: []diag.DiagnosisKey{
				{
					Key:       uuid.MustParse("adc69f96-c83f-4c2b-8905-ddf2b6ba8543"),
					DayNumber: uint16(42),
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

			stmt, err := tx.PrepareContext(ctx, "INSERT INTO diagnosis_keys (key, day_number) VALUES ($1, $2)")
			if err != nil {
				t.Fatal(err)
			}
			defer stmt.Close()

			for _, diagKey := range tt.diagKeys {
				_, err = stmt.ExecContext(ctx, diagKey.Key, diagKey.DayNumber)
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
