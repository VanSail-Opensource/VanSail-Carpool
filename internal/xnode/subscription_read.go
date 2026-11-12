package xnode

import (
	"context"
	"errors"
	"time"
)

// SubscriptionReadFence captures the authority used by one credential-bearing
// response. Its private identity snapshot retains hashes instead of material.
type SubscriptionReadFence struct {
	service        *Service
	identity       subscriptionIdentityState
	owner          subscriptionOwnerState
	version, local int64
	boundary       time.Time
}

// BeginSubscriptionRead must precede inventory, readiness and URL reads. The
// supplied identity is the exact snapshot the caller intends to publish.
func (s *Service) BeginSubscriptionRead(ctx context.Context, identity Identity) (SubscriptionReadFence, error) {
	repo, ok := s.repo.(compileVersionRepository)
	if !ok {
		return SubscriptionReadFence{}, ErrUnavailable
	}
	now := s.now()
	fence := SubscriptionReadFence{service: s, local: s.localCompileRevision.Load()}
	var err error
	fence.version, fence.boundary, err = repo.XNodeCompileVersion(ctx, now)
	if err != nil {
		return SubscriptionReadFence{}, err
	}
	current, err := s.repo.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil {
		return SubscriptionReadFence{}, err
	}
	fence.identity = subscriptionIdentitySnapshot(identity)
	if fresh := subscriptionIdentitySnapshot(current); fence.identity != fresh && !subscriptionProofGained(fence.identity, fresh) {
		return SubscriptionReadFence{}, ErrConflict
	}
	var boundary time.Time
	fence.owner, boundary, err = s.subscriptionReadOwner(ctx, current, now)
	if err != nil {
		return SubscriptionReadFence{}, err
	}
	if !boundary.IsZero() && (fence.boundary.IsZero() || boundary.Before(fence.boundary)) {
		fence.boundary = boundary
	}
	if err = s.checkSubscriptionReadEpoch(ctx, fence); err != nil {
		if errors.Is(err, errSubscriptionReadContention) {
			return fence, err // Retain the owner anchor for a complete-read retry.
		}
		return SubscriptionReadFence{}, err
	}
	return fence, nil
}

// CheckSubscriptionRead must run after the last slow read or rendering step and
// before returning any URL, host link or document. A failed check discards the
// whole response; the caller may retry the complete read with a fresh identity.
func (s *Service) CheckSubscriptionRead(ctx context.Context, fence SubscriptionReadFence) error {
	if fence.service != s || fence.identity.proxyID <= 0 {
		return ErrConflict
	}
	current, err := s.repo.XNodeIdentity(ctx, fence.identity.proxyID)
	if err != nil {
		return err
	}
	fresh := subscriptionIdentitySnapshot(current)
	gainedProof := subscriptionProofGained(fence.identity, fresh)
	if fence.identity != fresh && !gainedProof {
		return ErrConflict
	}
	owner, _, err := s.subscriptionReadOwner(ctx, current, s.now())
	if err != nil {
		return err
	}
	if owner != fence.owner {
		return ErrConflict
	}
	if err = s.checkSubscriptionReadEpoch(ctx, fence); err != nil {
		return err
	}
	if gainedProof {
		return errSubscriptionReadContention
	}
	return nil
}

func (s *Service) checkSubscriptionReadEpoch(ctx context.Context, fence SubscriptionReadFence) error {
	repo, ok := s.repo.(compileVersionRepository)
	if !ok {
		return ErrUnavailable
	}
	version, _, err := repo.XNodeCompileVersion(ctx, s.now())
	if err != nil {
		return err
	}
	if version != fence.version || s.localCompileRevision.Load() != fence.local ||
		!fence.boundary.IsZero() && !s.now().Before(fence.boundary) {
		return errSubscriptionReadContention
	}
	return nil
}
