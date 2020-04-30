package diag

import (
	"io"
	"sync"
)

// Cache defines an interface for caching binary Diagnosis Key data, to be used
// in between clients and the repository for listing keys.
type Cache interface {
	// Set replaces the cache.
	Set([]byte)
	// Add appends to the cache.
	Add([]byte)
	// WriteTo writes the cache to an io.Writer.
	WriteTo(w io.Writer) (int64, error)
	// Size returns the byte size of the cache.
	Size() int
}

// MemoryCache represents an in-memory cache.
// Uses a r/w mutex to ensure data integrity when the cache is being updated.
type MemoryCache struct {
	buf []byte
	mu  sync.RWMutex
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

// WriteTo writes the contents of the cache to an io.Writer.
func (mc *MemoryCache) WriteTo(w io.Writer) (int64, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	n, err := w.Write(mc.buf)
	return int64(n), err
}

// Size returns the cache size.
func (mc *MemoryCache) Size() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return len(mc.buf)
}
