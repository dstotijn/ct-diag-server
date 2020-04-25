package diag

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
)

// MaxUploadBatchSize is the maximum amount of diagnosis keys to be
// uploaded per request.
const MaxUploadBatchSize = 14

var (
	// ErrNilDiagKeys is used when an empty diagnosis keyset is used.
	ErrNilDiagKeys = errors.New("diag: diagnosis key array cannot be empty")
	// ErrMaxUploadExceeded is used when upload batch size exceeds the limit.
	ErrMaxUploadExceeded = errors.New("diag: maximum upload batch size exceeded")
)

// DiagnosisKey represents a `Daily Tracing Key` uploaded when the device owner
// is diagnosed positive.
//
// A diagnosis key takes up 18 bytes of data: 16 bytes for the key, 2 bytes for
// the day number.
//
// @see https://covid19-static.cdn-apple.com/applications/covid19/current/static/contact-tracing/pdf/ContactTracing-BluetoothSpecificationv1.1.pdf
type DiagnosisKey struct {
	Key       [16]byte
	DayNumber uint16 // Using uint16 saves data, but it will break the server on Jun 7, 2149.
}

// Repository defines an interface for storing and retrieving diagnosis keys
// in a repository.
type Repository interface {
	StoreDiagnosisKeys(context.Context, []DiagnosisKey) error
	FindAllDiagnosisKeys(context.Context) ([]DiagnosisKey, error)
}

// Service represents the service for managing diagnosis keys.
type Service struct {
	repo Repository
}

// NewService returns a new Service.
func NewService(repo Repository) Service {
	return Service{repo: repo}
}

// StoreDiagnosisKeys persists a set of diagnosis keys to the repository.
func (s Service) StoreDiagnosisKeys(ctx context.Context, diagKeys []DiagnosisKey) error {
	return s.repo.StoreDiagnosisKeys(ctx, diagKeys)
}

// FindAllDiagnosisKeys fetches all diagnosis keys from the repository.
func (s Service) FindAllDiagnosisKeys(ctx context.Context) ([]DiagnosisKey, error) {
	return s.repo.FindAllDiagnosisKeys(ctx)
}

// ParseDiagnosisKeys reads and parses diagnosis keys from an io.Reader.
func (s Service) ParseDiagnosisKeys(r io.Reader) ([]DiagnosisKey, error) {
	diagKeys := make([]DiagnosisKey, 0, MaxUploadBatchSize)

	for {
		// 18 bytes for the key (16 bytes) and the day number (2 bytes).
		var buf [18]byte
		_, err := io.ReadFull(r, buf[:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if len(diagKeys) == MaxUploadBatchSize {
			return nil, ErrMaxUploadExceeded
		}

		var key [16]byte
		copy(key[:], buf[:16])
		dayNumber := binary.BigEndian.Uint16(buf[16:])

		diagKeys = append(diagKeys, DiagnosisKey{Key: key, DayNumber: dayNumber})
	}

	return diagKeys, nil
}
