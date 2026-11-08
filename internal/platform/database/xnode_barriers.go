package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"math"
)

func (s *Store) UserByUsername(ctx context.Context, name string) (model.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, userSelect+` WHERE CAST(users.telegram_id AS TEXT)=$1`, name))
}
func (s *Store) XNodePeriods(ctx context.Context, user int64) ([]xnode.Period, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM xnode_periods WHERE proxy_user_id=$1 ORDER BY started_at,id`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		keys = append(keys, id)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	out := []xnode.Period{}
	for _, id := range keys {
		item, err := s.XNodePeriod(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// Track is written before granting admission, never after a potentially successful send.
func (s *Store) retryAttempt_TrackXNodePeriodNode(ctx context.Context, period, node string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	// This row lock serializes admission registration with the closing update.
	// An earlier registration is visible to closure; a later grant cannot send.
	err = tx.QueryRowContext(ctx, `SELECT status FROM xnode_periods WHERE id=$1 FOR UPDATE`, period).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return xnode.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "open" {
		return xnode.ErrConflict
	}
	var admitting bool
	err = tx.QueryRowContext(ctx, `SELECT removing_at IS NULL AND retired_at IS NULL FROM xnode_nodes WHERE id=$1 FOR KEY SHARE`, node).Scan(&admitting)
	if err != nil {
		return err
	}
	if !admitting {
		return xnode.ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_period_nodes(period_id,node_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, period, node); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) XNodePeriodNodes(ctx context.Context, period string) ([]xnode.PeriodNode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node_id,flushed_sequence FROM xnode_period_nodes WHERE period_id=$1 ORDER BY node_id`, period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.PeriodNode{}
	for rows.Next() {
		var item xnode.PeriodNode
		var sequence sql.NullInt64
		if err = rows.Scan(&item.NodeID, &sequence); err != nil {
			return nil, err
		}
		item.Flushed = sequence.Valid
		item.Sequence = uint64(sequence.Int64)
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) SealXNodePeriodNode(ctx context.Context, period, node string, sequence uint64) error {
	if sequence > math.MaxInt64 {
		return xnode.ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `UPDATE xnode_period_nodes SET flushed_sequence=$1 WHERE period_id=$2 AND node_id=$3 AND flushed_sequence IS NULL`, sequence, period, node)
	return err
}
func (s *Store) retryAttempt_ReopenXNodePeriod(ctx context.Context, id string) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE xnode_periods SET status='open',ended_at=NULL WHERE id=$1 AND status IN('closed','closing')`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return xnode.ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_period_nodes SET flushed_sequence=NULL WHERE period_id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}
