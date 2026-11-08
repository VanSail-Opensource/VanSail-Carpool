package xnode

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"time"
)

type nodeRemovalRepository interface {
	XNodeCompileVersion(context.Context, time.Time) (int64, time.Time, error)
	PrepareXNodeRemoval(context.Context, string, int64, ...RoutingPlan) error
	FinishXNodeRemoval(context.Context, string, int64, string, string) (bool, error)
	XNodeRemovalPeriods(context.Context, string) ([]Period, error)
	WaitXNodeRemoval(context.Context, string, int, string, time.Time) error
}

func (s *Service) removalVersion(ctx context.Context, id string) (int64, error) {
	repo, ok := s.repo.(nodeRemovalRepository)
	if !ok {
		return 0, ErrUnavailable
	}
	version, _, err := repo.XNodeCompileVersion(ctx, s.now())
	if err != nil {
		return 0, err
	}
	return version, nil
}

// Removal is an existing durable node operation, not a new wire command. Slow
// apply/traffic barriers run off the claim loop and never block traffic ACKs.
func (s *Service) runNodeRemoval(ctx context.Context, operation providerops.Operation, id string) error {
	_, ok := s.repo.(nodeRemovalRepository)
	operations, yes := s.repo.(operationRepository)
	if !ok || !yes || s.master == nil {
		return ErrUnavailable
	}
	operation, err := operations.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, s.now())
	if err != nil {
		return err
	}
	retired, removeErr := s.removeNode(ctx, id, operation.ActorUserID, operation.Receipt.ID)
	completion := providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: `{"removed":true,"retired":false}`}
	if retired {
		completion.ResultJSON = `{"removed":true,"retired":true}`
	}
	if removeErr != nil && !errors.Is(removeErr, ErrInvalid) {
		code := "XNODE_REMOVAL_SYNC_PENDING"
		var policy *PolicyError
		if errors.As(removeErr, &policy) {
			code = policy.Code
		}
		finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := s.repo.(nodeRemovalRepository).WaitXNodeRemoval(finish, operation.Receipt.ID, operation.Attempts, code, s.now()); err != nil {
			return err
		}
		return &outbox.WaitError{Reason: code, Cause: removeErr, After: time.Minute}
	}
	if removeErr != nil {
		completion.Status = providerops.StatusFailed
		completion.ErrorCode = "XNODE_REMOVAL_INPUT_INVALID"
	}
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err = operations.CompleteXNodeProviderOperation(finish, operation.Receipt.ID, operation.Attempts, completion, s.now())
	return err
}

func (s *Service) removeNode(ctx context.Context, id, actor, operation string) (bool, error) {
	repo := s.repo.(nodeRemovalRepository)
	node, err := s.repo.XNodeNode(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if node.RetiredAt != nil {
		return true, nil
	}
	s.mu.Lock()
	version, err := s.removalVersion(ctx, id)
	var changes []RoutingPlan
	if err == nil {
		changes, err = s.prepareRemovalRoutes(ctx, id)
	}
	if err == nil {
		err = repo.PrepareXNodeRemoval(ctx, id, version, changes...)
	}
	s.mu.Unlock()
	if err != nil {
		return false, err
	}
	if err = s.drainRemovalRoutes(ctx, id); err != nil {
		return false, err
	}
	m := s.master
	m.mu.RLock()
	peer := m.peers[id]
	m.mu.RUnlock()
	if peer != nil {
		if err = m.Sync(ctx, id); err != nil {
			return false, err
		}
		if err = m.WaitApplied(ctx, id); err != nil {
			return false, err
		}
		if _, err = m.Flush(ctx, id, 0, true); err != nil {
			return false, err
		}
	} else if node.AgentID != "" {
		return false, &PolicyError{"XNODE_REMOVAL_OFFLINE", ErrConflict}
	}
	periods, err := repo.XNodeRemovalPeriods(ctx, id)
	if err != nil {
		return false, err
	}
	for _, period := range periods {
		if peer == nil {
			return false, &PolicyError{"XNODE_REMOVAL_PENDING_ACCOUNTING", ErrConflict}
		}
		if err = s.flushPeriodProbes(ctx, period.ID, id); err != nil {
			return false, err
		}
		barrier, err := m.FlushPeriod(ctx, id, period.ProxyUserID, period.ID)
		if err != nil {
			return false, err
		}
		if err = s.repo.SealXNodeRemovalPeriodNode(ctx, period, id, barrier); err != nil {
			return false, err
		}
	}
	if err = m.CloseHourlyCreditObligations(ctx, "", -1, id); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	version, err = s.removalVersion(ctx, id)
	if err != nil {
		return false, err
	}
	// Pin the exact drained peer through revocation, excluding handshake races.
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.peers[id] != peer {
		return false, &PolicyError{"XNODE_REMOVAL_OFFLINE", ErrConflict}
	}
	retired, err := repo.FinishXNodeRemoval(ctx, id, version, actor, operation)
	if err == nil {
		m.disconnectPeerLocked(id)
		delete(m.live, id)
	}
	return retired, err
}

// RemovalBlocked exposes stable blocker codes without database details.
func RemovalBlocked(code string) error { return &PolicyError{code, ErrConflict} }
