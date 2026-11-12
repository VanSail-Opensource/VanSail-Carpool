package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// DeleteXNodeNode is compatibility cleanup for an unused disabled node. Enrolled
// or historical nodes must use the durable removal operation and its barriers.
func (s *Store) retryAttempt_DeleteXNodeNode(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Match the queued removal lock order. The node lock also serializes a
	// first agent bind and FK-backed dependencies with the eligibility check.
	var version int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM xnode_compile_revision WHERE singleton=1 FOR UPDATE`).Scan(&version); err != nil {
		return err
	}
	var unused bool
	err = tx.QueryRowContext(ctx, `SELECT enabled=0 AND COALESCE(agent_id,'')=''
 AND desired_revision=0 AND accepted_revision=0 AND applied_revision=0
 AND retired_at IS NULL AND removing_at IS NULL
 FROM xnode_nodes WHERE id=$1 FOR UPDATE`, id).Scan(&unused)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !unused {
		return xnode.ErrConflict
	}
	historical, err := nodeHasHistory(ctx, tx, id)
	if err != nil {
		return err
	}
	if historical {
		return xnode.ErrConflict
	}
	var endpoints bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_endpoints WHERE node_id=$1)`, id).Scan(&endpoints); err != nil {
		return err
	}
	if endpoints {
		return xnode.ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM xnode_nodes WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}
