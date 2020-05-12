// Package postgres provides an implementation of diag.Repository using PostgreSQL
// for underlying database storage.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dstotijn/ct-diag-server/diag"

	// Register pq for use via database/sql.
	_ "github.com/lib/pq"
)

// Client implements diag.Repository.
type Client struct {
	db                *sql.DB
	lastKnownKeyCount int
}

// New returns a new Client.
func New(dsn string) (*Client, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(30)

	return &Client{db: db}, nil
}

// Ping uses the underlying database client to for check connectivity.
func (c *Client) Ping() error {
	return c.db.Ping()
}

// Close uses the underlying database client to close all connections.
func (c *Client) Close() error {
	return c.db.Close()
}

// StoreDiagnosisKeys persists an array of diagnosis keys in the database.
func (c *Client) StoreDiagnosisKeys(ctx context.Context, diagKeys []diag.DiagnosisKey, uploadedAt time.Time) error {
	if len(diagKeys) == 0 {
		return diag.ErrNilDiagKeys
	}

	if uploadedAt.IsZero() {
		return errors.New("postgres: uploadedAt cannot be zero")
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: could not start transaction: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO diagnosis_keys (temporary_exposure_key, rolling_start_number, transmission_risk_level, uploaded_at) VALUES ($1, $2, $3, $4)
	ON CONFLICT ON CONSTRAINT diagnosis_keys_pkey DO NOTHING`)
	if err != nil {
		return fmt.Errorf("postgres: could not prepare statement: %v", err)
	}
	defer stmt.Close()

	for _, diagKey := range diagKeys {
		_, err = stmt.ExecContext(ctx,
			diagKey.TemporaryExposureKey[:],
			diagKey.RollingStartNumber,
			diagKey.TransmissionRiskLevel,
			uploadedAt,
		)
		if err != nil {
			return fmt.Errorf("postgres: could not execute statement: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: cannot commit transaction: %v", err)
	}

	return nil
}

// FindAllDiagnosisKeys finds all the Diagnosis Keys and returns them in their
// binary representation in a buffer.
func (c *Client) FindAllDiagnosisKeys(ctx context.Context) ([]diag.DiagnosisKey, error) {
	// Reduce the amount of allocs by anticipating the needed slice capacity.
	diagKeys := make([]diag.DiagnosisKey, 0, c.lastKnownKeyCount)

	query := `SELECT temporary_exposure_key, rolling_start_number, transmission_risk_level
	FROM diagnosis_keys
	ORDER BY index ASC`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("postgres: could not execute query: %v", err)
	}
	defer rows.Close()

	var rowCount int
	for rows.Next() {
		rowCount++
		var diagKey diag.DiagnosisKey
		key := diagKey.TemporaryExposureKey[:0]
		err := rows.Scan(&key, &diagKey.RollingStartNumber, &diagKey.TransmissionRiskLevel)
		if err != nil {
			return nil, fmt.Errorf("postgres: could not scan row: %v", err)
		}
		copy(diagKey.TemporaryExposureKey[:], key)
		diagKey.UploadedAt = diagKey.UploadedAt.In(time.UTC)

		diagKeys = append(diagKeys, diagKey)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: could not iterate over rows: %v", err)
	}

	c.lastKnownKeyCount = rowCount

	return diagKeys, nil
}

// LastModified returns the timestamp of the latest uploaded Diagnosis Key.
func (c *Client) LastModified(ctx context.Context) (time.Time, error) {
	var lastModified time.Time
	query := `SELECT uploaded_at FROM diagnosis_keys ORDER BY index DESC LIMIT 1`

	err := c.db.QueryRowContext(ctx, query).Scan(&lastModified)
	if err == sql.ErrNoRows {
		return time.Time{}, diag.ErrNilDiagKeys
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("postgres: could not execute query: %v", err)
	}

	return lastModified, nil
}
