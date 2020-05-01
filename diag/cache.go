package diag

import (
	"sync"
)

// Cache defines an interface for caching binary Diagnosis Key data, to be used
// in between clients and the repository for listing keys.
type Cache interface {
	// Get retrieves the contents from the cache.
	// Underlying data should *not* be mutated.
	Get() []byte
	// Set replaces the cache.
	Set([]byte)
	// Add appends to the cache.
	Add([]byte)
	// Size returns the byte size of the cache.
	Size() int
}

// MemoryCache represents an in-memory cache.
// The mutex is used to prevent concurrent appending to the slice.
type MemoryCache struct {
	buf []byte
	mu  sync.Mutex
}

// Get returns the buffer from the cache.
func (mc *MemoryCache) Get() []byte {
	return mc.buf
}

// Set overwrites the cache.
func (mc *MemoryCache) Set(buf []byte) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.buf = buf
}

// Add adds items to the cache.
func (mc *MemoryCache) Add(buf []byte) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.buf = append(mc.buf, buf...)
}

// Size returns the cache size.
func (mc *MemoryCache) Size() int {
	return len(mc.buf)
}
