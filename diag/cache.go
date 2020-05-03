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
	// LastModified returns the timestamp of the latest uploaded Diagnosis Key.
	LastModified() time.Time
	// ReadSeeker returns a io.ReadSeeker for accessing the cache. When a non zero
	// value is given for `since`, implementors should return Diagnosis Keys
	// from that timestamp (truncated by day) onwards.
	ReadSeeker(since time.Time) io.ReadSeeker
}

// MemoryCache represents an in-memory cache.
// The mutex is used to prevent concurrent appending to the slice and mutating
// the day offset map..
type MemoryCache struct {
	buf          []byte
	dayOffsets   map[time.Time]int
	lastModified time.Time
	mu           sync.Mutex
}

// Set overwrites the cache.
func (mc *MemoryCache) Set(diagKeys []DiagnosisKey, lastModified time.Time) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	buf := bytes.NewBuffer(make([]byte, 0, len(diagKeys)*DiagnosisKeySize))
	dayOffsets := make(map[time.Time]int)

	for i := range diagKeys {
		day := diagKeys[i].UploadedAt.Truncate(24 * time.Hour)
		if _, found := dayOffsets[day]; !found {
			dayOffsets[day] = buf.Len()
		}
		if err := writeDiagnosisKeys(buf, diagKeys[i]); err != nil {
			return err
		}
	}

	mc.buf = buf.Bytes()
	mc.dayOffsets = dayOffsets
	mc.lastModified = lastModified

	return nil
}

// Add appends items to the cache.
// Assumes that `diagKeys` is indexed with Diagnosis Keys ordered
// by upload timestamp, ascending.
func (mc *MemoryCache) Add(diagKeys []DiagnosisKey, lastModified time.Time) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	buf := bytes.NewBuffer(make([]byte, 0, len(diagKeys)*DiagnosisKeySize))
	for i := range diagKeys {
		day := diagKeys[i].UploadedAt.Truncate(24 * time.Hour)
		if _, found := mc.dayOffsets[day]; !found {
			mc.dayOffsets[day] = len(mc.buf) + buf.Len()
		}
		if err := writeDiagnosisKeys(buf, diagKeys[i]); err != nil {
			return err
		}
	}

	if err := writeDiagnosisKeys(buf, diagKeys...); err != nil {
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

// ReadSeeker returns a io.ReadSeeker for accessing Diagnosis Keys. When a non
// zero value `since` is passed, only Diagnosis Keys uploaded from that timestamp
// (truncated by day) will be returned. Else, all contents are used.
func (mc *MemoryCache) ReadSeeker(since time.Time) io.ReadSeeker {
	if since.IsZero() || len(mc.dayOffsets) == 0 {
		return bytes.NewReader(mc.buf)
	}

	since = since.Truncate(24 * time.Hour)

	var oldestDay time.Time
	var newestDay time.Time
	for day := range mc.dayOffsets {
		if oldestDay.IsZero() || day.Before(oldestDay) {
			oldestDay = day
		}
		if newestDay.IsZero() || day.After(newestDay) {
			newestDay = day
		}
	}

	switch {
	case since.Before(oldestDay):
		// Use all data.
		return bytes.NewReader(mc.buf)
	case since.After(newestDay):
		// Date in the future; use no data.
		return bytes.NewReader([]byte{})
	case since.Equal(newestDay):
		return bytes.NewReader(mc.buf[mc.dayOffsets[newestDay]:])
	default:
		for day := since; day.Before(newestDay.Add(24 * time.Hour)); day = day.Add(24 * time.Hour) {
			if offset, found := mc.dayOffsets[day]; found {
				return bytes.NewReader(mc.buf[offset:])
			}
		}
		// Should never be reached, but needed by compiler.
		return bytes.NewReader([]byte{})
	}
}
