package xnodemigration

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

func (s *Service) sourceConfig(state xnode.MigrationState) (SourceConfig, error) {
	var config SourceConfig
	raw, err := s.vault.Decrypt("xnode.migration.source", state.SourceCipher)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal([]byte(raw), &config)
	return config, err
}
func checkpoint(state xnode.MigrationState, id string) (xnode.MigrationNode, error) {
	for _, node := range state.Nodes {
		if node.NodeID == id {
			return node, nil
		}
	}
	return xnode.MigrationNode{}, xnode.ErrNotFound
}

// Rollback removes native listeners and reconciles their journal before returning
// the node to the legacy provider. It never replays a database backup.
func (s *Service) Rollback(ctx context.Context, id string) (xnode.MigrationNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.repo.XNodeMigration(ctx)
	if err != nil {
		return xnode.MigrationNode{}, err
	}
	node, err := checkpoint(state, id)
	if err != nil {
		return node, err
	}
	if node.Phase == "legacy" {
		return node, nil
	}
	if state.Phase != "active" || (node.Phase != "native" && node.Phase != "preparing" && node.Phase != "rolling_back" && node.Phase != "review") {
		return node, xnode.ErrConflict
	}
	return s.rollbackNode(ctx, state, node)
}
func (s *Service) rollbackNode(ctx context.Context, state xnode.MigrationState, node xnode.MigrationNode) (xnode.MigrationNode, error) {
	source, err := s.sourceConfig(state)
	if err != nil {
		return node, err
	}
	node.Phase = "rolling_back"
	if err = s.repo.SaveXNodeMigrationNode(ctx, node); err != nil {
		return node, err
	}
	if err = s.master.Sync(ctx, node.NodeID); err != nil {
		return node, err
	}
	child, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = s.master.WaitApplied(child, node.NodeID)
	cancel()
	if err != nil {
		return node, err
	}
	if _, err = s.master.Flush(ctx, node.NodeID, 0, true); err != nil {
		return node, err
	}
	users, err := s.repo.MigrationPanelAccounts(ctx)
	if err != nil {
		return node, err
	}
	// SyncUsage also compiles each remaining legacy quota from the combined usage.
	if err = s.source.SyncUsage(ctx, source, users); err != nil {
		return node, err
	}
	if err = s.source.SetNodeDisabled(ctx, source, node.LegacyNodeID, false); err != nil {
		return node, err
	}
	node.Phase, node.ErrorCode, node.UsageCutoff = "legacy", "", nil
	return node, s.repo.SaveXNodeMigrationNode(ctx, node)
}
