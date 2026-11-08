package xnode

import (
	"context"
	"errors"
	"fmt"
)

// A closing period admits no new participants. Registration is fenced by the
// same database row update, before any grant can be sent to an agent.
func (s *Service) closePeriod(ctx context.Context, period Period, allowPending ...bool) error {
	// A settled predecessor is immutable and never needs another admission or
	// network read when its purchase is replayed alongside a paid successor.
	if period.Status == "closed" {
		return nil
	}
	canContinue := len(allowPending) > 0 && allowPending[0]
	current, err := s.repo.XNodePeriod(ctx, period.ID)
	if err != nil {
		return err
	}
	period = current
	if period.Status == "closed" {
		return nil
	}
	if period.Status == "review" {
		return ErrConflict
	}
	if period.Status == "open" {
		period.Status = "closing"
		at := s.now().UTC()
		period.EndedAt = &at
		if err := s.repo.SaveXNodePeriod(ctx, period); err != nil {
			return err
		}
	}
	if err := s.revokePeriodProbes(ctx, period.ID); err != nil {
		return err
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
	if writer, ok := s.repo.(interface {
		FinalizeXNodePeriod(context.Context, Period) error
	}); ok {
		return writer.FinalizeXNodePeriod(ctx, period)
	}
	return s.repo.SaveXNodePeriod(ctx, period)
}

func (s *Service) closePeriodNode(ctx context.Context, period Period, node string) error {
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
	barrier, err := s.master.FlushPeriod(ctx, node, period.ProxyUserID, period.ID)
	if err != nil {
		return fmt.Errorf("journal_barrier_pending: %w", err)
	}
	if barrier.StreamID != "" {
		streams, ok := s.repo.(TrafficStreamRepository)
		if !ok {
			return ErrUnavailable
		}
		return streams.SealXNodeStreamPeriodNode(ctx, period.ID, node, barrier)
	}
	return s.repo.SealXNodePeriodNode(ctx, period.ID, node, barrier.Sequence)
}
