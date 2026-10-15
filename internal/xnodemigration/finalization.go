package xnodemigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"path/filepath"
	"strings"
	"time"
)

// StageFinal captures complete source history after all source nodes are disabled.
// The operator supplies the verified final source-backup reference for review.
func (s *Service) StageFinal(ctx context.Context, backupReference string) (xnode.MigrationState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.repo.XNodeMigration(ctx)
	if err != nil {
		return state, err
	}
	if state.Phase != "active" || strings.TrimSpace(backupReference) == "" || len(backupReference) > 512 {
		return state, xnode.ErrConflict
	}
	if err = allNative(state); err != nil {
		return state, err
	}
	source, err := s.sourceConfig(state)
	if err != nil {
		return state, err
	}
	users, err := s.repo.MigrationPanelAccounts(ctx)
	if err != nil {
		return state, err
	}
	var initial Snapshot
	raw, err := s.vault.Decrypt("xnode.migration.snapshot", state.SnapshotCipher)
	if err != nil || json.Unmarshal([]byte(raw), &initial) != nil {
		return state, xnode.ErrInvalid
	}
	if err = s.source.SyncUsage(ctx, source, users); err != nil {
		return state, err
	}
	snapshot, err := s.source.Capture(ctx, source, users, initial.Monitoring)
	if err != nil {
		return state, err
	}
	if err = sourceDisabled(state, snapshot.Nodes); err != nil {
		return state, err
	}
	if err = s.reconcileFinalSnapshot(ctx, snapshot); err != nil {
		return state, err
	}
	data, err := json.Marshal(snapshot)
	if err != nil || len(data) > 32<<20 {
		return state, xnode.ErrInvalid
	}
	digest := sha256.Sum256(data)
	state.FinalDigest = hex.EncodeToString(digest[:])
	state.FinalBackupReference = backupReference
	state.FinalSnapshotCipher, err = s.vault.Encrypt("xnode.migration.final", string(data))
	if err != nil {
		return state, err
	}
	return state, s.repo.SaveXNodeMigration(ctx, state)
}

func (s *Service) Finalize(ctx context.Context, digest string) (xnode.MigrationState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.repo.XNodeMigration(ctx)
	if err != nil {
		return state, err
	}
	if state.Phase == "complete" && state.FinalDigest == digest {
		return state, nil
	}
	if state.Phase != "active" || state.FinalDigest != digest || digest == "" {
		return state, xnode.ErrConflict
	}
	if err = allNative(state); err != nil {
		return state, err
	}
	for _, node := range state.Nodes {
		if err = s.master.WaitApplied(ctx, node.NodeID); err != nil {
			return state, err
		}
	}
	backup, err := s.backups.Run(ctx)
	if err != nil || backup.Status != "complete" {
		return state, xnode.ErrUnavailable
	}
	state.BackupName = filepath.Base(backup.Path)
	var snapshot Snapshot
	raw, err := s.vault.Decrypt("xnode.migration.final", state.FinalSnapshotCipher)
	if err != nil || json.Unmarshal([]byte(raw), &snapshot) != nil {
		return state, xnode.ErrInvalid
	}
	source, err := s.sourceConfig(state)
	if err != nil {
		return state, err
	}
	// No listeners are stopped. The existing queue drains earlier source writes
	// before this short check; purchases resume with native ownership after commit.
	err = s.source.WithProvisioningFence(ctx, func(ctx context.Context) error {
		child, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		nodes, accounts, err := s.source.FinalAccounts(child, source)
		if err != nil {
			return err
		}
		if err = sourceDisabled(state, nodes); err != nil {
			return err
		}
		if err = s.verifyFinalAccounts(child, snapshot, accounts); err != nil {
			return err
		}
		at := s.panel.Now()
		state.Phase = "complete"
		state.FinalizedAt = &at
		return s.repo.SaveXNodeMigration(child, state)
	})
	return state, err
}

func allNative(state xnode.MigrationState) error {
	if len(state.Nodes) == 0 {
		return xnode.ErrConflict
	}
	for _, node := range state.Nodes {
		if node.Phase != "native" {
			return xnode.ErrConflict
		}
	}
	return nil
}
func sourceDisabled(state xnode.MigrationState, nodes []Node) error {
	found := map[string]bool{}
	for _, node := range nodes {
		if node.Enabled {
			return xnode.ErrConflict
		}
		found[node.ID] = true
	}
	for _, node := range state.Nodes {
		if !found[node.LegacyNodeID] {
			return xnode.ErrConflict
		}
	}
	return nil
}
