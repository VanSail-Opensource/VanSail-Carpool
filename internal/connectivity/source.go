package connectivity

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

func (s *Service) loadTargets(ctx context.Context, cfg Config, hash string, fresh bool) (User, []Target, error) {
	s.mu.RLock()
	generation := s.generation
	s.mu.RUnlock()
	if !fresh {
		if err := s.cachedFailure(hash); err != nil {
			return User{}, nil, err
		}
		if user, targets, ok := s.cachedTargets(hash); ok {
			return user, targets, nil
		}
	}
	select {
	case s.sourceGate <- struct{}{}:
		defer func() { <-s.sourceGate }()
	case <-ctx.Done():
		return User{}, nil, ctx.Err()
	}
	if !fresh {
		if err := s.cachedFailure(hash); err != nil {
			return User{}, nil, err
		}
		if user, targets, ok := s.cachedTargets(hash); ok {
			return user, targets, nil
		}
	}
	sourceCtx, cancel := context.WithTimeout(ctx, sourceTimeout)
	user, targets, err := s.source.Load(sourceCtx, cfg.RemnawaveUserID)
	if err == nil && sourceCtx.Err() != nil {
		err = sourceCtx.Err()
	}
	cancel()
	if err == nil {
		err = validateSourceResult(cfg.RemnawaveUserID, user, targets)
	}
	s.mu.Lock()
	current := s.hash == hash && s.generation == generation && ctx.Err() == nil
	if !current && err == nil {
		err = &CodeError{Code: "CONNECTIVITY_INTERRUPTED"}
	}
	if current {
		s.attemptedAt, s.attemptHash = s.now().UTC(), hash
		if err != nil {
			s.loadError = ErrorCode(err)
		} else {
			copy := user
			s.user, s.targets = &copy, metadataTargets(targets)
			s.loadedAt, s.loadHash, s.loadError = s.now().UTC(), hash, ""
		}
	}
	s.mu.Unlock()
	return user, cloneTargets(targets), err
}

func (s *Service) cachedFailure(hash string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.attemptHash == hash && s.loadError != "" && !s.attemptedAt.IsZero() && s.now().Sub(s.attemptedAt) < sourceCacheTTL {
		return &CodeError{Code: s.loadError}
	}
	return nil
}

func (s *Service) cachedTargets(hash string) (User, []Target, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.user == nil || s.loadHash != hash || s.loadedAt.IsZero() || s.loadError != "" || s.now().Sub(s.loadedAt) >= sourceCacheTTL {
		return User{}, nil, false
	}
	return *s.user, cloneTargets(s.targets), true
}

func validateSourceResult(userID int64, user User, targets []Target) error {
	if user.ID != userID || user.ID <= 0 || strings.TrimSpace(user.Username) == "" || user.Status != "ACTIVE" {
		return &CodeError{Code: "CONNECTIVITY_ACCOUNT_UNAVAILABLE"}
	}
	known := make(map[string]bool, len(targets))
	for _, target := range targets {
		_, legacyIDError := uuid.Parse(target.HostUUID)
		invalidNativeID := target.HostUUID == "" || len(target.HostUUID) > 256 || strings.ContainsFunc(target.HostUUID, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
		if invalidNativeID || (len(target.NativeOutbound) == 0 && legacyIDError != nil) || known[target.HostUUID] {
			return &CodeError{Code: "CONNECTIVITY_SUBSCRIPTION_UNAVAILABLE"}
		}
		known[target.HostUUID] = true
	}
	return nil
}

func cloneTargets(targets []Target) []Target {
	result := append([]Target{}, targets...)
	for index := range result {
		result[index].Resolved = append([]byte(nil), result[index].Resolved...)
		result[index].NativeOutbound = append([]byte(nil), result[index].NativeOutbound...)
	}
	return result
}

func metadataTargets(targets []Target) []Target {
	result := append([]Target{}, targets...)
	for index := range result {
		result[index].Resolved = nil
		result[index].NativeOutbound = nil
	}
	return result
}

func (s *Service) cacheState(hash string) (*User, []Target, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.user == nil || s.loadHash != hash {
		return nil, []Target{}, s.loadError
	}
	user := *s.user
	return &user, cloneTargets(s.targets), s.loadError
}

func (s *Service) runState(hash string) (*Run, bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var run *Run
	if s.pending != nil && s.pending.hash == hash && s.pending.generation == s.generation {
		copy := cloneRun(s.pending.run)
		run = &copy
	} else if s.active != nil && s.active.hash == hash && s.active.generation == s.generation {
		copy := cloneRun(s.active.run)
		run = &copy
	} else if s.last != nil && s.lastHash == hash {
		copy := cloneRun(*s.last)
		run = &copy
	}
	return run, s.ready, s.storageError
}

// RetentionWindow is the maximum age of diagnostic attempts returned or kept.
const RetentionWindow = 24 * time.Hour
