package diag

import "context"

// Repository defines an interface for storing and retrieving diagnosis keys
// in a repository.
type Repository interface {
	StoreDiagnosisKeys(context.Context, []DiagnosisKey) error
	FindAllDiagnosisKeys(context.Context) ([]DiagnosisKey, error)
}
