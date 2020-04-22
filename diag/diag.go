package diag

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrNilDiagKeys is used when an empty diagnosis keyset is used.
var ErrNilDiagKeys = errors.New("diagnosis key array cannot be empty")

// DiagnosisKey represents a `Daily Tracing Key` uploaded when the device owner
// is diagnosed positive.
//
// A diagnosis key takes up 18 bytes of data: 16 bytes for the key, 2 bytes for
// the day number.
//
// @see https://covid19-static.cdn-apple.com/applications/covid19/current/static/contact-tracing/pdf/ContactTracing-BluetoothSpecificationv1.1.pdf
type DiagnosisKey struct {
	Key       uuid.UUID
	DayNumber uint16 // Using uint16 saves data, but it will break the server on Jun 7, 2149.
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
