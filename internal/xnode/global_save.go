package xnode

import (
	"context"
	"encoding/json"
	"time"
)

type globalWriteFence struct {
	version int64
	until   time.Time
}

// Remote validation uses a private candidate. Only the short publication phase
// holds the writer lock, and PostgreSQL fences every durable compile input.
func (s *Service) SaveGlobal(ctx context.Context, actor string, next GlobalConfig) error {
	s.mu.Lock()
	previous, err := s.Global(ctx)
	var fence globalWriteFence
	if err == nil {
		fence, err = s.captureGlobalWrite(ctx)
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	next, err = prepareGlobalChange(previous, next)
	if err != nil {
		return err
	}
	if err = s.validateGlobalChange(ctx, previous, next); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commitGlobalLocked(ctx, actor, previous, next, fence)
}

func (s *Service) captureGlobalWrite(ctx context.Context) (globalWriteFence, error) {
	if repo, ok := s.repo.(compileVersionRepository); ok {
		version, until, err := repo.XNodeCompileVersion(ctx, s.now().UTC())
		return globalWriteFence{version: version, until: until}, err
	}
	return globalWriteFence{}, nil
}

func (s *Service) commitGlobalLocked(ctx context.Context, actor string, previous, next GlobalConfig, fence globalWriteFence) error {
	current, err := s.Global(ctx)
	if err != nil {
		return err
	}
	if globalDocumentKey(current) != globalDocumentKey(previous) || (!fence.until.IsZero() && !s.now().Before(fence.until)) {
		return ErrConflict
	}
	latest, err := s.captureGlobalWrite(ctx)
	if err != nil {
		return err
	}
	if latest.version != fence.version {
		return ErrConflict
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	committed, err := DecodeGlobal(string(raw))
	if err != nil {
		return err
	}
	if err = s.settings.PutXNodeGlobalAtVersion(ctx, actor, string(raw), fence.version, fence.until); err != nil {
		return err
	}
	s.globalSettings.publish(committed)
	if s.master != nil {
		s.master.configureAutomaticLeases(committed)
	}
	if repo, ok := s.repo.(interface{ ResetFirewallDetection() }); ok {
		before, _ := json.Marshal(previous.FirewallDetection)
		after, _ := json.Marshal(next.FirewallDetection)
		if string(before) != string(after) {
			repo.ResetFirewallDetection()
		}
	}
	if globalRuntimeKey(previous) != globalRuntimeKey(next) {
		s.localCompileRevision.Add(1)
	}
	return nil
}
