package database

import (
	"context"
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) retryAttempt_SetXNodeRate(ctx context.Context, nodeID, agent string, cutoff uint64, multiplier int64) error {
	if multiplier < 0 || cutoff >= math.MaxInt64 {
		return xnode.ErrInvalid
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var bound string
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(agent_id,'') FROM xnode_nodes WHERE id=?`, nodeID).Scan(&bound); err != nil {
		return err
	}
	if bound != agent {
		return xnode.ErrConflict
	}
	var committed uint64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT last_sequence FROM xnode_report_cursors WHERE agent_id=?),0)`, agent).Scan(&committed)
	if err != nil {
		return err
	}
	if committed < cutoff || committed >= math.MaxInt64 {
		return xnode.ErrConflict
	}
	// Reports committed after the flush retain their already-settled price.
	// Only the next uncommitted sequence may adopt the new multiplier.
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_node_rates(agent_id,from_sequence,multiplier_fp) VALUES(?,?,?) ON CONFLICT(agent_id,from_sequence) DO UPDATE SET multiplier_fp=excluded.multiplier_fp`, agent, committed+1, multiplier)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE xnode_nodes SET multiplier_fp=? WHERE id=?`, multiplier, nodeID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
