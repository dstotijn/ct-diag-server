package diag

import (
	"bytes"
	"io"
	"sync"
	"time"
)

// Cache defines an interface for caching binary Diagnosis Key data, to be used
// in between clients and the repository for listing keys.
type Cache interface {
	// Set replaces the cache.
	Set(diagKeys []DiagnosisKey, lastModified time.Time) error
	// Add appends to the cache.
	Add(diagKeys []DiagnosisKey, lastModified time.Time) error
	// Size returns the byte size of the cache.
	Size() int
	// LastModified returns the timestamp of the latest uploaded Diagnosis Key.
	LastModified() time.Time
	// ReadSeeker returns a io.ReadSeeker for accessing the cache.
	ReadSeeker() io.ReadSeeker
}

// MemoryCache represents an in-memory cache.
// The mutex is used to prevent concurrent appending to the slice.
type MemoryCache struct {
	buf          []byte
	lastModified time.Time
	mu           sync.Mutex
}

// Set overwrites the cache.
func (mc *MemoryCache) Set(diagKeys []DiagnosisKey, lastModified time.Time) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	buf := bytes.NewBuffer(make([]byte, 0, len(diagKeys)*DiagnosisKeySize))
	if err := writeDiagnosisKeys(buf, diagKeys); err != nil {
		return err
	}

	mc.buf = buf.Bytes()
	mc.lastModified = lastModified

	return nil
}

// Add appends items to the cache.
func (mc *MemoryCache) Add(diagKeys []DiagnosisKey, lastModified time.Time) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	buf := bytes.NewBuffer(make([]byte, 0, len(diagKeys)*DiagnosisKeySize))
	if err := writeDiagnosisKeys(buf, diagKeys); err != nil {
		return err
	}

	mc.buf = append(mc.buf, buf.Bytes()...)
	mc.lastModified = lastModified

	return nil
}

// Size returns the cache size.
func (mc *MemoryCache) Size() int {
	return len(mc.buf)
}

// LastModified returns the timestamp of the latest uploaded Diagnosis Key in the cache.
func (mc *MemoryCache) LastModified() time.Time {
	return mc.lastModified
}

// ReadSeeker returns a new bytes.Reader, which implements io.ReadSeeker.
func (mc *MemoryCache) ReadSeeker() io.ReadSeeker {
	return bytes.NewReader(mc.buf)
}
