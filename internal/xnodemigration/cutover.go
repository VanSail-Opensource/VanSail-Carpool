package xnodemigration

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

// Cutover keeps the old provider serving throughout preflight. After its disable
// has been attempted, every failure restores ownership or records required review.
func (s *Service) Cutover(ctx context.Context, id string) (node xnode.MigrationNode, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.repo.XNodeMigration(ctx)
	if err != nil {
		return node, err
	}
	node, err = checkpoint(state, id)
	if err != nil {
		return node, err
	}
	if state.Phase != "active" {
		return node, xnode.ErrConflict
	}
	if node.Phase == "native" {
		err = s.master.Sync(ctx, id)
		if err == nil {
			err = s.master.WaitApplied(ctx, id)
		}
		return node, err
	}
	if node.Phase != "legacy" && node.Phase != "preparing" {
		return node, xnode.ErrConflict
	}
	// An interrupted attempt may have disabled the source before recording native
	// ownership. Restore its known serving state before trying another handover.
	if node.Phase == "preparing" {
		node, err = s.rollbackNode(ctx, state, node)
		if err != nil {
			return node, err
		}
	}
	if _, err = s.panel.VerifyRelease(ctx, id); err != nil {
		return node, err
	}
	if err = s.panel.VerifyClientEndpoints(ctx, id); err != nil {
		return node, err
	}
	candidate, err := s.panel.Candidate(ctx, id)
	if err != nil {
		return node, err
	}
	if _, err = s.master.Command(ctx, id, "", "config.validate", candidate); err != nil {
		return node, err
	}
	backup, err := s.backups.Run(ctx)
	if err != nil || backup.Status != "complete" {
		return node, errors.Join(err, xnode.ErrUnavailable)
	}
	source, err := s.sourceConfig(state)
	if err != nil {
		return node, err
	}
	node.Phase = "preparing"
	if err = s.repo.SaveXNodeMigrationNode(ctx, node); err != nil {
		return node, err
	}
	defer func() {
		if err == nil {
			return
		}
		recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 90*time.Second)
		defer cancel()
		restored, rollbackErr := s.rollbackNode(recovery, state, node)
		if rollbackErr == nil {
			node = restored
			return
		}
		node.Phase, node.ErrorCode = "review", "XNODE_ROLLBACK_PENDING"
		err = errors.Join(err, rollbackErr, s.repo.SaveXNodeMigrationNode(recovery, node))
	}()
	// Install recovery before the call: a lost response can still mean disabled.
	if err = s.source.SetNodeDisabled(ctx, source, node.LegacyNodeID, true); err != nil {
		return node, err
	}
	users, err := s.repo.MigrationPanelAccounts(ctx)
	if err != nil {
		return node, err
	}
	if err = s.source.SyncUsage(ctx, source, users); err != nil {
		return node, err
	}
	cutoff := time.Now().UTC()
	node.UsageCutoff, node.Phase, node.ErrorCode = &cutoff, "native", ""
	if err = s.repo.SaveXNodeMigrationNode(ctx, node); err != nil {
		return node, err
	}
	if err = s.master.Sync(ctx, id); err == nil {
		child, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = s.master.WaitApplied(child, id)
		cancel()
	}
	return node, err
}
