package xnode

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// A closing period admits no new participants. Registration is fenced by the
// same database row update, before any grant can be sent to an agent.
func (s *Service) closePeriod(ctx context.Context, period Period, allowPending ...bool) error {
	period, err := s.preparePeriodClosure(ctx, period)
	if err != nil {
		return err
	}
	return s.settlePeriodClosure(ctx, period, len(allowPending) > 0 && allowPending[0])
}

// preparePeriodClosure contains database work only and may run under Service.mu.
func (s *Service) preparePeriodClosure(ctx context.Context, period Period) (Period, error) {
	var at time.Time
	if period.Status != "closed" {
		at = s.now().UTC()
	}
	period, err := s.repo.BeginXNodePeriodClosure(ctx, period, at)
	if err != nil {
		return Period{}, err
	}
	if period.Status != "closed" {
		err = s.revokePeriodProbes(ctx, period.ID)
	}
	return period, err
}

// settlePeriodClosure waits only for this captured closure generation. Callers
// requiring a successor use canContinue and leave every network wait to recovery.
func (s *Service) settlePeriodClosure(ctx context.Context, period Period, canContinue bool) error {
	if period.Status == "closed" {
		return nil
	}
	participants, err := s.repo.XNodePeriodNodes(ctx, period.ID)
	if err != nil {
		return err
	}
	if canContinue {
		for _, node := range participants {
			if !node.Flushed {
				if s.master != nil {
					select {
					case s.master.stateWake <- struct{}{}:
					default:
					}
				}
				return nil
			}
		}
	}
	var pending error
	for _, node := range participants {
		if node.Flushed {
			continue
		}
		if err := s.closePeriodNode(ctx, period, node.NodeID); err != nil {
			pending = errors.Join(pending, fmt.Errorf("period %s node %s: %w", period.ID, node.NodeID, err))
		}
		if ctx.Err() != nil {
			return errors.Join(pending, ctx.Err())
		}
	}
	if pending != nil {
		return pending
	}
	// Never complete from a stale participant snapshot. Sealing each participant
	// remains subject to its acknowledged traffic journal, including old boots.
	participants, err = s.repo.XNodePeriodNodes(ctx, period.ID)
	if err != nil {
		return err
	}
	for _, node := range participants {
		if !node.Flushed {
			return fmt.Errorf("period %s node %s barrier_pending: %w", period.ID, node.NodeID, ErrUnavailable)
		}
	}
	period.Status = "closed"
	return s.repo.FinalizeXNodePeriod(ctx, period)
}

func (s *Service) closePeriodNode(ctx context.Context, period Period, node string) error {
	if err := s.checkPeriodClosure(ctx, period); err != nil {
		return err
	}
	if s.master == nil || !s.master.Connected(node) {
		return fmt.Errorf("participant_offline: %w", ErrUnavailable)
	}
	// Sync takes the peer's existing serialization lock. Any grant registered
	// before closing finishes its send before this revocation is published.
	if err := s.master.Sync(ctx, node); err != nil {
		return fmt.Errorf("revocation_pending: %w", err)
	}
	if err := s.flushPeriodProbes(ctx, period.ID, node); err != nil {
		return fmt.Errorf("probe_barrier_pending: %w", err)
	}
	if err := s.checkPeriodClosure(ctx, period); err != nil {
		return err
	}
	barrier, err := s.master.FlushPeriod(ctx, node, period.ProxyUserID, period.ID)
	if err != nil {
		return fmt.Errorf("journal_barrier_pending: %w", err)
	}
	return s.repo.SealXNodeClosingPeriodNode(ctx, period, node, barrier)
}

func (s *Service) checkPeriodClosure(ctx context.Context, expected Period) error {
	current, err := s.repo.XNodePeriod(ctx, expected.ID)
	if err != nil {
		return err
	}
	if current.Status != "closing" || !samePeriodClosure(current, expected) {
		return ErrConflict
	}
	return nil
}

func samePeriodClosure(current, expected Period) bool {
	return current.ID == expected.ID && current.ProxyUserID == expected.ProxyUserID && current.PurchaseID == expected.PurchaseID && current.ClosureGeneration == expected.ClosureGeneration
}
