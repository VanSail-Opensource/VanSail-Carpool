package database

import (
	"context"
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Store) SetXNodeStreamRate(ctx context.Context, node, agent, stream string, cutoff uint64, multiplier int64) error {
	if multiplier < 0 || cutoff >= math.MaxInt64 || !wire.ValidStreamID(stream) {
		return xnode.ErrInvalid
	}
	for attempt := 0; ; attempt++ {
		err := s.setXNodeStreamRate(ctx, node, agent, stream, cutoff, multiplier)
		if attempt >= 3 || !s.retryTransaction(ctx, err, attempt) {
			return err
		}
	}
}

func (s *Store) setXNodeStreamRate(ctx context.Context, node, agent, stream string, cutoff uint64, multiplier int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var bound string
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(agent_id,'') FROM xnode_nodes WHERE id=$1 FOR UPDATE`, node).Scan(&bound); err != nil {
		return err
	}
	if bound != agent {
		return xnode.ErrConflict
	}
	committed, _, closed, err := lockXNodeStreamCursorTx(ctx, tx, node, agent, stream)
	if err != nil {
		return err
	}
	if closed || committed < cutoff || committed >= math.MaxInt64 {
		return xnode.ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_stream_rates(agent_id,stream_id,from_sequence,multiplier_fp) VALUES($1,$2,$3,$4) ON CONFLICT(agent_id,stream_id,from_sequence) DO UPDATE SET multiplier_fp=excluded.multiplier_fp`, agent, stream, committed+1, multiplier); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_nodes SET multiplier_fp=$1 WHERE id=$2`, multiplier, node); err != nil {
		return err
	}
	return tx.Commit()
}
