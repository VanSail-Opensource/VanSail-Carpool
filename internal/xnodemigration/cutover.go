package xnodemigration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

type cutoverFailure struct {
	code string
	err  error
}

func (e *cutoverFailure) Error() string { return e.err.Error() }
func (e *cutoverFailure) Unwrap() error { return e.err }

func cutoverFailed(code string, err error) error {
	if err == nil {
		return nil
	}
	return &cutoverFailure{code: code, err: err}
}

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
		return node, cutoverFailed("XNODE_CUTOVER_AGENT_APPLY_FAILED", err)
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
		return node, cutoverFailed("XNODE_CUTOVER_RELEASE_CHECK_FAILED", err)
	}
	source, err := s.sourceConfig(state)
	if err != nil {
		return node, cutoverFailed("XNODE_CUTOVER_SOURCE_CONFIG_FAILED", err)
	}
	topology, err := s.source.EndpointTopology(ctx, source)
	if err != nil {
		return node, cutoverFailed("XNODE_CUTOVER_SOURCE_TOPOLOGY_FAILED", err)
	}
	if repair, ok := s.panel.(interface {
		RepairImportedTemplates(context.Context, string, map[string]json.RawMessage) error
	}); ok {
		if err = repair.RepairImportedTemplates(ctx, id, topology.HostTemplates); err != nil {
			return node, cutoverFailed("XNODE_CUTOVER_TEMPLATE_REPAIR_FAILED", err)
		}
	}
	if err = s.panel.VerifyClientEndpoints(ctx, id); err != nil {
		return node, cutoverFailed("XNODE_CUTOVER_ENDPOINT_CHECK_FAILED", err)
	}
	candidate, err := s.panel.Candidate(ctx, id)
	if err != nil {
		return node, cutoverFailed("XNODE_CUTOVER_CANDIDATE_BUILD_FAILED", err)
	}
	if _, err = s.master.Command(ctx, id, "", "config.validate", candidate); err != nil {
		return node, cutoverFailed("XNODE_CUTOVER_AGENT_VALIDATION_FAILED", err)
	}
	native := map[string]bool{}
	for _, peer := range state.Nodes {
		native[peer.LegacyNodeID] = peer.Phase == "native"
	}
	issues, topologyErr := sharedSS2022Hosts(topology, node.LegacyNodeID, native)
	if topologyErr != nil {
		return node, cutoverFailed("XNODE_CUTOVER_HOST_TOPOLOGY_INVALID", topologyErr)
	}
	if len(issues) > 0 {
		return node, errors.Join(ErrSharedEndpoint, xnode.ErrConflict)
	}
	if err = s.verifyNodeAccounts(ctx, source, node.LegacyNodeID, candidate.Users); err != nil {
		return node, cutoverFailed("XNODE_CUTOVER_ACCOUNT_PREFLIGHT_FAILED", err)
	}
	backup, err := s.backups.Run(ctx)
	if err != nil || backup.Status != "complete" {
		return node, cutoverFailed("XNODE_CUTOVER_BACKUP_FAILED", errors.Join(err, xnode.ErrUnavailable))
	}
	node.Phase = "preparing"
	if err = s.repo.SaveXNodeMigrationNode(ctx, node); err != nil {
		return node, cutoverFailed("XNODE_CUTOVER_CHECKPOINT_FAILED", err)
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
		return node, cutoverFailed("XNODE_CUTOVER_SOURCE_DISABLE_FAILED", err)
	}
	users, err := s.repo.MigrationPanelAccounts(ctx)
	if err != nil {
		return node, cutoverFailed("XNODE_CUTOVER_ACCOUNT_LOAD_FAILED", err)
	}
	if err = s.source.SyncUsage(ctx, source, users); err != nil {
		return node, cutoverFailed("XNODE_CUTOVER_USAGE_SYNC_FAILED", err)
	}
	cutoff := time.Now().UTC()
	node.UsageCutoff, node.Phase, node.ErrorCode = &cutoff, "native", ""
	if err = s.repo.SaveXNodeMigrationNode(ctx, node); err != nil {
		return node, cutoverFailed("XNODE_CUTOVER_NATIVE_CHECKPOINT_FAILED", err)
	}
	if err = s.master.Sync(ctx, id); err == nil {
		child, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = s.master.WaitApplied(child, id)
		cancel()
	}
	return node, cutoverFailed("XNODE_CUTOVER_AGENT_APPLY_FAILED", err)
}
