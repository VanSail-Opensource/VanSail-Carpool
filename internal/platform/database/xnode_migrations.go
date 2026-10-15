package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) XNodeMigration(ctx context.Context) (xnode.MigrationState, error) {
	result := xnode.MigrationState{Phase: "none", Nodes: []xnode.MigrationNode{}}
	var finalized sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT phase,source_cipher,snapshot_cipher,snapshot_digest,source_backup_reference,backup_name,final_snapshot_cipher,final_digest,final_source_backup_reference,finalized_at FROM xnode_migrations WHERE singleton=1`).Scan(&result.Phase, &result.SourceCipher, &result.SnapshotCipher, &result.SnapshotDigest, &result.SourceBackupReference, &result.BackupName, &result.FinalSnapshotCipher, &result.FinalDigest, &result.FinalBackupReference, &finalized)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if finalized.Valid {
		at, err := time.Parse(time.RFC3339Nano, finalized.String)
		if err != nil {
			return result, err
		}
		result.FinalizedAt = &at
	}
	rows, err := s.db.QueryContext(ctx, `SELECT node_id,legacy_node_id,phase,source_digest,usage_cutoff,error_code FROM xnode_migration_nodes ORDER BY node_id`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var item xnode.MigrationNode
		var cutoff sql.NullString
		if err = rows.Scan(&item.NodeID, &item.LegacyNodeID, &item.Phase, &item.SourceDigest, &cutoff, &item.ErrorCode); err != nil {
			return result, err
		}
		if cutoff.Valid {
			value, e := time.Parse(time.RFC3339Nano, cutoff.String)
			if e != nil {
				return result, e
			}
			item.UsageCutoff = &value
		}
		result.Nodes = append(result.Nodes, item)
	}
	return result, rows.Err()
}
func (s *Store) SaveXNodeMigration(ctx context.Context, item xnode.MigrationState) error {
	var finalized any
	if item.FinalizedAt != nil {
		finalized = stamp(*item.FinalizedAt)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO xnode_migrations(singleton,phase,source_cipher,snapshot_cipher,snapshot_digest,source_backup_reference,backup_name,final_snapshot_cipher,final_digest,final_source_backup_reference,finalized_at,updated_at) VALUES(1,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(singleton) DO UPDATE SET phase=excluded.phase,source_cipher=excluded.source_cipher,snapshot_cipher=excluded.snapshot_cipher,snapshot_digest=excluded.snapshot_digest,source_backup_reference=excluded.source_backup_reference,backup_name=excluded.backup_name,final_snapshot_cipher=excluded.final_snapshot_cipher,final_digest=excluded.final_digest,final_source_backup_reference=excluded.final_source_backup_reference,finalized_at=excluded.finalized_at,updated_at=excluded.updated_at`, item.Phase, item.SourceCipher, item.SnapshotCipher, item.SnapshotDigest, item.SourceBackupReference, item.BackupName, item.FinalSnapshotCipher, item.FinalDigest, item.FinalBackupReference, finalized, stamp(time.Now().UTC()))
	return err
}
func (s *Store) SaveXNodeMigrationNode(ctx context.Context, item xnode.MigrationNode) error {
	var cutoff any
	if item.UsageCutoff != nil {
		cutoff = stamp(*item.UsageCutoff)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO xnode_migration_nodes(node_id,legacy_node_id,phase,source_digest,usage_cutoff,error_code,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(node_id) DO UPDATE SET phase=excluded.phase,source_digest=excluded.source_digest,usage_cutoff=excluded.usage_cutoff,error_code=excluded.error_code,updated_at=excluded.updated_at`, item.NodeID, item.LegacyNodeID, item.Phase, item.SourceDigest, cutoff, item.ErrorCode, stamp(time.Now().UTC()))
	return err
}

// ImportXNodeMigrationNode publishes the node and its legacy ownership together.
// A concurrent compiler can never observe a newly imported native listener.
func (s *Store) ImportXNodeMigrationNode(ctx context.Context, node xnode.Node, checkpoint xnode.MigrationNode) error {
	if node.ID != checkpoint.NodeID || checkpoint.Phase != "legacy" {
		return xnode.ErrInvalid
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Resumption preserves operator edits and already transferred ownership.
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_nodes(id,name,country_code,provider_name,multiplier_fp,enabled,config_id,settings_cipher,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, xnodeNodeArgs(node)...); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_migration_nodes(node_id,legacy_node_id,phase,source_digest,updated_at) VALUES(?,?,'legacy',?,?) ON CONFLICT(node_id) DO NOTHING`, node.ID, checkpoint.LegacyNodeID, checkpoint.SourceDigest, stamp(time.Now().UTC())); err != nil {
		return err
	}
	return tx.Commit()
}
