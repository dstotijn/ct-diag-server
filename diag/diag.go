// Package diag provides a service for parsing, storing and writing Diagnosis
// Keys. Because the server is load heavy, it has a cache interface to unburden
// the repository.
package diag

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"go.uber.org/zap"
)

// DiagnosisKeySize represents the size of a Diagnosis Key when transmitted
// over a network in bytes (16 bytes for the TemporaryExposure Key, 4 bytes
// for the RollingStartNumber, and 1 byte for the TransmissionRiskLevel).
const DiagnosisKeySize = 21

const defaultMaxUploadBatchSize = 14

var (
	// ErrNilDiagKeys is used when an empty diagnosis keyset is encountered.
	ErrNilDiagKeys = errors.New("diag: diagnosis keys is nil")

	// ErrMaxUploadExceeded is used when upload batch size exceeds the limit.
	ErrMaxUploadExceeded = errors.New("diag: maximum upload batch size exceeded")
)

// DiagnosisKey is a TemporaryExposure key with its related rollingStartNumber,
// and the timestamp of its submission to the server.
// @see https://developer.apple.com/documentation/exposurenotification/entemporaryexposurekey
type DiagnosisKey struct {
	TemporaryExposureKey  [16]byte
	RollingStartNumber    uint32
	TransmissionRiskLevel byte
	UploadedAt            time.Time
}

// Repository defines an interface for storing and retrieving diagnosis keys
// in a repository.
type Repository interface {
	StoreDiagnosisKeys(ctx context.Context, diagKeys []DiagnosisKey, createdAt time.Time) error
	FindAllDiagnosisKeys(ctx context.Context) ([]DiagnosisKey, error)
	LastModified(ctx context.Context) (time.Time, error)
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
	n, err := svc.cache.ReadSeeker(time.Time{}).Seek(0, io.SeekEnd)
	if err != nil {
		return Service{}, fmt.Errorf("diag: could not seek cache: %v", err)
	}
	svc.logger.Info("Cache hydrated.", zap.Int64("size", n))

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
	now := time.Now().UTC()

	if err := s.repo.StoreDiagnosisKeys(ctx, diagKeys, now); err != nil {
		return err
	}

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
		rollingStartNumber := binary.BigEndian.Uint32(buf[start+16 : start+DiagnosisKeySize])
		transRiskLevel := buf[start+20]

		diagKeys[i] = DiagnosisKey{
			TemporaryExposureKey:  key,
			RollingStartNumber:    rollingStartNumber,
			TransmissionRiskLevel: transRiskLevel,
		}
	}

	return diagKeys, nil
}

// ReadSeeker returns an io.ReadSeeker for accessing the cache.
// When a non zero `since` value is passed, Diagnosis Keys from that timestamp
// (truncated by day) onwards will be returned. Else, all contents are used.
func (s Service) ReadSeeker(since time.Time) io.ReadSeeker {
	return s.cache.ReadSeeker(since)
}

// LastModified returns the timestamp of the latest Diagnosis Key upload.
func (s Service) LastModified() time.Time {
	return s.cache.LastModified().UTC()
}

// MaxUploadBatchSize returns the maximum number of diagnosis keys to be uploaded
// per request.
func (s Service) MaxUploadBatchSize() uint {
	return s.maxUploadBatchSize
}

func writeDiagnosisKeys(w io.Writer, diagKeys ...DiagnosisKey) error {
	// Write binary data for the diagnosis keys. Per diagnosis key, 16 bytes are
	// written with the diagnosis key itself, and 4 bytes for `RollingStartNumber`
	// (uint32, big endian). Because both parts have a fixed length, there is no
	// delimiter.
	for i := range diagKeys {
		_, err := w.Write(diagKeys[i].TemporaryExposureKey[:])
		if err != nil {
			return err
		}
		rollingStartNumber := make([]byte, 4)
		binary.BigEndian.PutUint32(rollingStartNumber, diagKeys[i].RollingStartNumber)
		_, err = w.Write(rollingStartNumber)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte{byte(diagKeys[i].TransmissionRiskLevel)})
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

	lastModified, err := s.repo.LastModified(ctx)
	if err == ErrNilDiagKeys {
		return nil
	}
	if err != nil {
		return err
	}

	if err := s.cache.Set(diagKeys, lastModified); err != nil {
		return err
	}

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
				continue
			}
			n, err := s.cache.ReadSeeker(time.Time{}).Seek(0, io.SeekEnd)
			if err != nil {
				s.logger.Error("Could not seek cache", zap.Error(err))
				continue
			}

			s.logger.Info("Cache refreshed.", zap.Int64("size", n))
		}
	}
}
