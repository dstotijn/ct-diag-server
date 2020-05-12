// Package diag provides a service for parsing, storing and writing Diagnosis
// Keys. Because the server is load heavy, it has a cache interface to unburden
// the repository.
package diag

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/dstotijn/ct-diag-server/diag/pb"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// MaxUploadSize is the maximum size of uploads in bytes.
const MaxUploadSize = 500 * 1000

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

// ExposureConfig represents the parameters for detecting exposure.
// @see https://developer.apple.com/documentation/exposurenotification/enexposureconfiguration
type ExposureConfig struct {
	MinimumRiskScore                 uint8   `json:"minimumRiskScore"`
	AttenuationLevelValues           []int   `json:"attenuationLevelValues"`
	AttenuationWeight                float32 `json:"attenuationWeight"`
	DaysSinceLastExposureLevelValues []int   `json:"daysSinceLastExposureLevelValues"`
	DaysSinceLastExposureWeight      float32 `json:"daysSinceLastExposureWeight"`
	DurationLevelValues              []int   `json:"durationLevelValues"`
	DurationWeight                   float32 `json:"durationWeight"`
	TransmissionRiskLevelValues      []int   `json:"transmissionRiskLevelValues"`
	TransmissionRiskWeight           float32 `json:"transmissionRiskWeight"`
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
	repo   Repository
	cache  Cache
	logger *zap.Logger
}

// Config represents the configuration to create a Service.
type Config struct {
	Repository     Repository
	Cache          Cache
	CacheInterval  time.Duration
	Logger         *zap.Logger
	ExposureConfig ExposureConfig
}

// NewService returns a new Service.
func NewService(ctx context.Context, cfg Config) (Service, error) {
	if cfg.Logger == nil {
		return Service{}, errors.New("diag: logger cannot be nil")
	}
	svc := Service{
		repo:   cfg.Repository,
		cache:  cfg.Cache,
		logger: cfg.Logger,
	}

	// Default to in-memory cache.
	if svc.cache == nil {
		svc.cache = &MemoryCache{}
	}

	// Set sane default for cache refresh interval.
	if cfg.CacheInterval == 0 {
		cfg.CacheInterval = 5 * time.Minute
	}

	// Hydrate cache.
	n, err := svc.hydrateCache(ctx)
	if err != nil {
		return Service{}, fmt.Errorf("diag: could not hydrate cache: %v", err)
	}
	svc.logger.Info("Cache hydrated.", zap.Int("size", n))

	// Run cache refresh worker in separate goroutine.
	go func() {
		if err := svc.refreshCache(ctx, cfg.CacheInterval); err != nil && err != context.Canceled {
			svc.logger.Error("Could not refresh cache.", zap.Error(err))
		}
	}()

	return svc, nil
}

// ListDiagnosisKeys returns all available Diagnosis Keys.
func (s Service) ListDiagnosisKeys(after [16]byte) ([]DiagnosisKey, error) {
	diagKeys, err := s.cache.Get()
	if err != nil {
		return nil, fmt.Errorf("diag: could not get diagnosis keys from cache: %v", err)
	}

	if after == [16]byte{} {
		return diagKeys, nil
	}

	for i := range diagKeys {
		if diagKeys[i].TemporaryExposureKey == after {
			return diagKeys[i+1:], nil
		}
	}

	return nil, nil
}

// StoreDiagnosisKeys persists a set of diagnosis keys to the repository.
func (s Service) StoreDiagnosisKeys(ctx context.Context, diagKeys []DiagnosisKey) error {
	now := time.Now().UTC()

	if err := s.repo.StoreDiagnosisKeys(ctx, diagKeys, now); err != nil {
		return err
	}

	return nil
}

// ParseDiagnosisKeyFile reads and parses a Diagnosis Key protobuf from an io.Reader.
func ParseDiagnosisKeyFile(r io.Reader) ([]DiagnosisKey, error) {
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("diag: could not read diagnosis keys: %v", err)
	}

	diagKeyFile := &pb.File{}
	if err := proto.Unmarshal(buf, diagKeyFile); err != nil {
		return nil, fmt.Errorf("diag: could not decode protobuf: %v", err)
	}

	if len(diagKeyFile.GetKey()) == 0 {
		return nil, nil
	}

	diagKeys := make([]DiagnosisKey, len(diagKeyFile.GetKey()))
	for i := range diagKeyFile.Key {
		var tek [16]byte
		n := copy(tek[:], diagKeyFile.Key[i].KeyData)
		if n != 16 {
			return nil, fmt.Errorf("diag: unexpected key length (%d)", n)
		}

		rollingStartNumber := diagKeyFile.Key[i].GetRollingStartNumber()
		if rollingStartNumber == 0 {
			return nil, errors.New("diag: rollingStartNumber cannot be 0")
		}

		transRiskLevel := diagKeyFile.Key[i].GetTransmissionRiskLevel()
		if transRiskLevel > 255 {
			return nil, errors.New("diag: transmissionRiskLevel does not fit uint8 range")
		}

		diagKeys[i] = DiagnosisKey{
			TemporaryExposureKey:  tek,
			RollingStartNumber:    rollingStartNumber,
			TransmissionRiskLevel: uint8(transRiskLevel),
		}
	}

	return diagKeys, nil
}

// LastModified returns the timestamp of the latest Diagnosis Key upload.
func (s Service) LastModified() (time.Time, error) {
	lastModified, err := s.cache.LastModified()
	if err != nil {
		return time.Time{}, fmt.Errorf("diag: could not get last modified: %v", err)
	}

	return lastModified.UTC(), nil
}

// WriteDiagnosisKeyProtobuf writes Diagnosis Keys as a protobuf to an io.Writer,
// and returns the bytes written.
func WriteDiagnosisKeyProtobuf(w io.Writer, diagKeys ...DiagnosisKey) (int, error) {
	keyFile := &pb.File{
		Key: make([]*pb.Key, len(diagKeys)),
	}

	// TODO: Add `Header` message.

	for i := range diagKeys {
		keyFile.Key[i] = &pb.Key{
			KeyData:               diagKeys[i].TemporaryExposureKey[:],
			RollingPeriod:         proto.Uint32(42),
			RollingStartNumber:    proto.Uint32(diagKeys[i].RollingStartNumber),
			TransmissionRiskLevel: proto.Int32(int32(diagKeys[i].TransmissionRiskLevel)),
		}
	}

	buf, err := proto.Marshal(keyFile)
	if err != nil {
		return 0, fmt.Errorf("diag: could not encode to protobuf: %v", err)
	}

	return w.Write(buf)
}

func (s Service) hydrateCache(ctx context.Context) (int, error) {
	diagKeys, err := s.repo.FindAllDiagnosisKeys(ctx)
	if err != nil {
		return 0, err
	}

	lastModified, err := s.repo.LastModified(ctx)
	if err != nil && err != ErrNilDiagKeys {
		return 0, err
	}

	if err := s.cache.Set(diagKeys, lastModified); err != nil {
		return 0, err
	}

	return len(diagKeys), nil
}

func (s Service) refreshCache(ctx context.Context, interval time.Duration) error {
	t := time.NewTicker(interval)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			n, err := s.hydrateCache(ctx)
			if err != nil {
				s.logger.Error("Could not refresh cache", zap.Error(err))
				continue
			}

			s.logger.Info("Cache refreshed.", zap.Int("size", n))
		}
	}
}
