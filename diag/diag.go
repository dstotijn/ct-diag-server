// Package diag provides a service for parsing, storing and writing Diagnosis
// Keys. Because the server is load heavy, it has a cache interface to unburden
// the repository.
package diag

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"go.uber.org/zap"
)

const (
	// DiagnosisKeySize is the size of a `Diagnosis Key`, consisting of a
	// `TemporaryExposureKey` (16 bytes) and a `ENIntervalNumber` (4 bytes).
	DiagnosisKeySize = 20

	defaultMaxUploadBatchSize = 14
)

var (
	// ErrNilDiagKeys is used when an empty diagnosis keyset is used.
	ErrNilDiagKeys = errors.New("diag: diagnosis key array cannot be empty")

	// ErrMaxUploadExceeded is used when upload batch size exceeds the limit.
	ErrMaxUploadExceeded = errors.New("diag: maximum upload batch size exceeded")
)

// DiagnosisKey is the combination of a `TemporaryExposureKey` and its related
// `ENIntervalNumber`. In total, a DiagnosisKey takes up 20 bytes when sent over
// the wire. Note: The `ENIntervalNumber` is the 10 minute time window since Unix
// Epoch when the key `TemporaryExposureKey` was generated.
// @see https://covid19-static.cdn-apple.com/applications/covid19/current/static/contact-tracing/pdf/ExposureNotification-CryptographySpecificationv1.1.pdf
type DiagnosisKey struct {
	TemporaryExposureKey [16]byte
	ENIntervalNumber     uint32
}

// Repository defines an interface for storing and retrieving diagnosis keys
// in a repository.
type Repository interface {
	StoreDiagnosisKeys(context.Context, []DiagnosisKey) error
	FindAllDiagnosisKeys(context.Context) ([]DiagnosisKey, error)
}

// Service represents the service for managing diagnosis keys.
type Service struct {
	repo               Repository
	cache              Cache
	maxUploadBatchSize uint
	logger             *zap.Logger
}

// Config represents the configuration to create a Service.
type Config struct {
	Repository         Repository
	Cache              Cache
	MaxUploadBatchSize uint
	Logger             *zap.Logger
}

// NewService returns a new Service.
func NewService(ctx context.Context, cfg Config) (Service, error) {
	if cfg.Logger == nil {
		return Service{}, errors.New("diag: logger cannot be nil")
	}
	svc := Service{
		repo:               cfg.Repository,
		cache:              cfg.Cache,
		maxUploadBatchSize: cfg.MaxUploadBatchSize,
		logger:             cfg.Logger,
	}

	// Default to in-memory cache.
	if svc.cache == nil {
		svc.cache = &MemoryCache{}
	}

	// Set sane default for max upload batch size.
	if svc.maxUploadBatchSize == 0 {
		svc.maxUploadBatchSize = defaultMaxUploadBatchSize
	}

	// Hydrate cache.
	if err := svc.hydrateCache(ctx); err != nil {
		return Service{}, fmt.Errorf("diag: could not hydrate cache: %v", err)
	}
	svc.logger.Info("Cache hydrated.", zap.Int("size", svc.cache.Size()))

	// Run cache refresh worker in separate goroutine.
	go func() {
		if err := svc.refreshCache(ctx); err != nil && err != context.Canceled {
			svc.logger.Error("Could not refresh cache.", zap.Error(err))
		}
	}()

	return svc, nil
}

// StoreDiagnosisKeys persists a set of diagnosis keys to the repository.
func (s Service) StoreDiagnosisKeys(ctx context.Context, diagKeys []DiagnosisKey) error {
	if err := s.repo.StoreDiagnosisKeys(ctx, diagKeys); err != nil {
		return err
	}

	go func() {
		buf := bytes.NewBuffer(make([]byte, 0, len(diagKeys)*DiagnosisKeySize))
		writeDiagnosisKeys(buf, diagKeys)
		s.cache.Add(buf.Bytes())
		s.logger.Info("Stored new diagnosis keys.", zap.Int("count", len(diagKeys)))
	}()

	return nil
}

// FindAllDiagnosisKeys fetches all diagnosis keys from the repository.
func (s Service) FindAllDiagnosisKeys(ctx context.Context) ([]DiagnosisKey, error) {
	return s.repo.FindAllDiagnosisKeys(ctx)
}

// ParseDiagnosisKeys reads and parses diagnosis keys from an io.Reader.
func ParseDiagnosisKeys(r io.Reader) ([]DiagnosisKey, error) {
	buf, err := ioutil.ReadAll(r)
	n := len(buf)

	switch {
	case err != nil && err != io.EOF:
		return nil, err
	case n == 0:
		return nil, io.ErrUnexpectedEOF
	case n%DiagnosisKeySize != 0:
		return nil, io.ErrUnexpectedEOF
	}

	keyCount := n / DiagnosisKeySize
	diagKeys := make([]DiagnosisKey, keyCount)

	for i := 0; i < keyCount; i++ {
		start := i * DiagnosisKeySize
		var key [16]byte
		copy(key[:], buf[start:start+16])
		enin := binary.BigEndian.Uint32(buf[start+16 : start+DiagnosisKeySize])

		diagKeys[i] = DiagnosisKey{TemporaryExposureKey: key, ENIntervalNumber: enin}
	}

	return diagKeys, nil
}

// ItemCount returns the amount of known diagnosis keys.
func (s Service) ItemCount() int {
	return s.cache.Size() / DiagnosisKeySize
}

// MaxUploadBatchSize returns the maximum number of diagnosis keys to be uploaded
// per request.
func (s Service) MaxUploadBatchSize() uint {
	return s.maxUploadBatchSize
}

// WriteDiagnosisKeys writes a stream of Diagnosis Keys to an io.Writer.
func (s Service) WriteDiagnosisKeys(w io.Writer) (n int, err error) {
	buf := s.cache.Get()
	n, err = w.Write(buf)
	s.logger.Debug("Finished writing Diagnosis Keys.",
		zap.Int("bytesWritten", n),
		zap.Error(err),
	)
	return
}

func writeDiagnosisKeys(w io.Writer, diagKeys []DiagnosisKey) error {
	// Write binary data for the diagnosis keys. Per diagnosis key, 16 bytes are
	// written with the diagnosis key itself, and 4 bytes for `ENIntervalNumber`
	// (uint32, big endian). Because both parts have a fixed length, there is no
	// delimiter.
	for i := range diagKeys {
		_, err := w.Write(diagKeys[i].TemporaryExposureKey[:])
		if err != nil {
			return err
		}
		enin := make([]byte, 4)
		binary.BigEndian.PutUint32(enin, diagKeys[i].ENIntervalNumber)
		_, err = w.Write(enin)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s Service) hydrateCache(ctx context.Context) error {
	diagKeys, err := s.repo.FindAllDiagnosisKeys(ctx)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(make([]byte, 0, len(diagKeys)*DiagnosisKeySize))
	writeDiagnosisKeys(buf, diagKeys)
	s.cache.Set(buf.Bytes())

	return nil
}

func (s Service) refreshCache(ctx context.Context) error {
	t := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if err := s.hydrateCache(ctx); err != nil {
				s.logger.Error("Could not refresh cache", zap.Error(err))
			}
			s.logger.Info("Cache refreshed.", zap.Int("size", s.cache.Size()))
		}
	}
}
