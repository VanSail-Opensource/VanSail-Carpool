package xnode

import (
	"context"
	"errors"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Each native phase names a captured old tuple and rechecks admission. Network
// waits never hold the account row or the service mutex.
func (s *Service) settleAccountPurgePeriod(ctx context.Context, repo accountNativePurgeRepository, target model.AccountPurge, jobID string, period Period) error {
	if period.Status == "closed" {
		return nil
	}
	if err := s.accountPurgeStillAdmitted(ctx, repo, target); err != nil {
		return err
	}
	if err := s.revokePeriodProbes(ctx, period.ID); err != nil {
		return err
	}
	participants, err := s.repo.XNodePeriodNodes(ctx, period.ID)
	if err != nil {
		return err
	}
	var pending error
	for _, node := range participants {
		if node.Flushed {
			continue
		}
		if err = s.closeAccountPurgePeriodNode(ctx, repo, target, jobID, period, node.NodeID); err != nil {
			pending = errors.Join(pending, fmt.Errorf("node %s: %w", node.NodeID, err))
		}
		if ctx.Err() != nil {
			return errors.Join(pending, ctx.Err())
		}
	}
	if pending != nil {
		return pending
	}
	if err = s.accountPurgeStillAdmitted(ctx, repo, target); err != nil {
		return err
	}
	// Lifecycle and participant/cursor evidence are rechecked in one transaction.
	_, err = repo.FinalizeAccountNativePurgePeriod(ctx, target, jobID, period)
	return err
}

func (s *Service) closeAccountPurgePeriodNode(ctx context.Context, repo accountNativePurgeRepository, target model.AccountPurge, jobID string, period Period, node string) error {
	if err := s.accountPurgeStillAdmitted(ctx, repo, target); err != nil {
		return err
	}
	if err := s.checkPeriodClosure(ctx, period); err != nil {
		return err
	}
	if s.master == nil || !s.master.Connected(node) {
		return fmt.Errorf("participant_offline: %w", ErrUnavailable)
	}
	if err := s.master.Sync(ctx, node); err != nil {
		return fmt.Errorf("revocation_pending: %w", err)
	}
	if err := s.accountPurgeStillAdmitted(ctx, repo, target); err != nil {
		return err
	}
	if err := s.flushPeriodProbes(ctx, period.ID, node); err != nil {
		return fmt.Errorf("probe_barrier_pending: %w", err)
	}
	barrier, err := s.master.FlushPeriod(ctx, node, period.ProxyUserID, period.ID)
	if err != nil {
		return fmt.Errorf("journal_barrier_pending: %w", err)
	}
	if err = s.accountPurgeStillAdmitted(ctx, repo, target); err != nil {
		return err
	}
	_, err = repo.SealAccountNativePurgePeriodNode(ctx, target, jobID, period, node, barrier)
	return err
}
