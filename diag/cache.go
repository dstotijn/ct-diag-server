package diag

import (
	"time"
)

// Cache defines an interface for caching binary Diagnosis Key data, to be used
// in between clients and the repository for listing keys.
type Cache interface {
	// Set replaces the cache.
	Set(diagKeys []DiagnosisKey, lastModified time.Time) error
	// Get returns the contents of the cache.
	Get() ([]DiagnosisKey, error)
	// LastModified returns the timestamp of the latest uploaded Diagnosis Key.
	LastModified() (time.Time, error)
}

// MemoryCache represents an in-memory cache.
type MemoryCache struct {
	diagKeys     []DiagnosisKey
	lastModified time.Time
}

// Set overwrites the cache.
func (mc *MemoryCache) Set(diagKeys []DiagnosisKey, lastModified time.Time) error {
	mc.diagKeys = diagKeys
	mc.lastModified = lastModified

	return nil
}

// Get returns the cache contents. The underlying array storage should not be
// mutated by callers.
func (mc *MemoryCache) Get() ([]DiagnosisKey, error) {
	return mc.diagKeys, nil
}

// LastModified returns the timestamp of the latest uploaded Diagnosis Key in the cache.
func (mc *MemoryCache) LastModified() (time.Time, error) {
	return mc.lastModified, nil
}
