package usdt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

// Service coordinates the single-process collector and local-only public reads.
type Service struct {
	repository  Repository
	settings    Settings
	executor    *Executor
	mu          sync.RWMutex
	saveMu      sync.Mutex
	fingerprint string
	states      map[string]NetworkStatus
	done        chan struct{}
	start       sync.Once
	wake        chan struct{}
	OnError     func(string)
}

// NewService uses the existing encrypted settings facade and SQLite repository.
func NewService(repository Repository, settings Settings) *Service {
	return &Service{repository: repository, settings: settings, executor: newExecutor(), states: map[string]NetworkStatus{}, done: make(chan struct{}), wake: make(chan struct{}, 1)}
}

func (s *Service) config(ctx context.Context) (Config, error) {
	raw, err := s.settings.Optional(ctx, ConfigKey)
	if err != nil {
		return Config{}, err
	}
	return DecodeConfig(raw)
}
func fingerprint(c Config) string {
	raw, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// Config returns a write-only credential projection; reads never contact a chain.
func (s *Service) Config(ctx context.Context) (Config, error) {
	c, err := s.config(ctx)
	if err != nil {
		return Config{}, err
	}
	return PublicConfig(c), nil
}

// SaveConfig preserves blank secrets, audits an encrypted write and invalidates
// previous readiness immediately. Existing quote recipients stay monitored.
func (s *Service) SaveConfig(ctx context.Context, actor string, next Config) (Config, error) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	old, err := s.config(ctx)
	if err != nil {
		return Config{}, err
	}
	mergeSecrets(&next, old)
	if err := ValidateConfig(&next); err != nil {
		return Config{}, err
	}
	for _, spec := range Registry {
		addresses, err := s.repository.USDTWatchAddresses(ctx, spec.Network)
		if err != nil {
			return Config{}, err
		}
		if len(addresses) == 0 {
			continue
		}
		found := false
		for _, n := range next.Networks {
			if n.Network == spec.Network && len(n.Endpoints) > 0 {
				found = true
			}
		}
		if !found {
			return Config{}, ErrConfiguration
		}
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return Config{}, err
	}
	if err := s.settings.Put(ctx, actor, ConfigKey, string(raw)); err != nil {
		return Config{}, err
	}
	s.mu.Lock()
	s.fingerprint = ""
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return PublicConfig(next), nil
}

// Review and Resolve consume only persisted, verified transfer evidence.
func (s *Service) Review(ctx context.Context, cursor string, limit int) (ReviewPage, error) {
	return s.repository.USDTReview(ctx, cursor, limit)
}
func (s *Service) Resolve(ctx context.Context, id, orderID, actor, reason string) (model.PaymentOrder, bool, error) {
	return s.repository.SettleUSDTTransfer(ctx, id, orderID, actor, reason)
}

// ErrorCode returns safe identifiers without upstream URLs or response text.
func ErrorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrRPC.Error()
	}
	for _, known := range []error{ErrConfiguration, ErrUnavailable, ErrCapacity, ErrConflict, ErrRPC, ErrProof, ErrQuoteNotFound} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	if errors.Is(err, context.Canceled) {
		return "USDT_INTERRUPTED"
	}
	return "USDT_STORAGE_UNAVAILABLE"
}
