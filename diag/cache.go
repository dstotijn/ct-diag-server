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
	// Get retrieves the contents from the cache.
	// Underlying data should *not* be mutated.
	Get() []byte
	// Set replaces the cache.
	Set(buf []byte, lastModified time.Time)
	// Add appends to the cache.
	Add(buf []byte, lastModified time.Time)
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

// Get returns the buffer from the cache.
func (mc *MemoryCache) Get() []byte {
	return mc.buf
}

// Set overwrites the cache.
func (mc *MemoryCache) Set(buf []byte, lastModified time.Time) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.buf = buf
	mc.lastModified = lastModified
}

// Add adds items to the cache.
func (mc *MemoryCache) Add(buf []byte, lastModified time.Time) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.buf = append(mc.buf, buf...)
	mc.lastModified = lastModified
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
