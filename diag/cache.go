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
	// LastModified returns the timestamp of the latest uploaded Diagnosis Key.
	LastModified() time.Time
	// ReadSeeker returns a io.ReadSeeker for accessing the cache. When a non zero
	// value is given for `after`, implementors should use Diagnosis Keys
	// uploaded after the given key, else all Diagnosis Keys should be used..
	ReadSeeker(after [16]byte) io.ReadSeeker
}

// MemoryCache represents an in-memory cache.
// The mutex is used to prevent concurrent appending to the slice and mutating
// the day offset map..
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

	if err := writeDiagnosisKeys(buf, diagKeys...); err != nil {
		return err
	}

	mc.buf = buf.Bytes()
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

// ReadSeeker returns a io.ReadSeeker for accessing Diagnosis Keys. When a non
// zero `after` is passed, only Diagnosis Keys uploaded after the given key
// will be returned. Else, all contents are used.
func (mc *MemoryCache) ReadSeeker(after [16]byte) io.ReadSeeker {
	if after == [16]byte{} {
		return bytes.NewReader(mc.buf)
	}

	// Look for the key in the buffer.
	for i := 0; i < len(mc.buf); i = i + DiagnosisKeySize {
		if bytes.Equal(mc.buf[i:i+16], after[:]) {
			// The key was found. The offset becomes the index *after* this key.
			return bytes.NewReader(mc.buf[i+DiagnosisKeySize:])
		}
	}

	// Key was not found. Use an empty reader.
	return bytes.NewReader([]byte{})
}
