package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Store) RegisterXNodeTrafficStream(ctx context.Context, node, agent, stream string) error {
	for attempt := 0; ; attempt++ {
		err := s.registerXNodeTrafficStream(ctx, node, agent, stream)
		if attempt >= 3 || !s.retryTransaction(ctx, err, attempt) {
			return err
		}
	}
}

func (s *Store) registerXNodeTrafficStream(ctx context.Context, node, agent, stream string) error {
	if stream != "" && !wire.ValidStreamID(stream) {
		return xnode.ErrInvalid
	}
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
		return xnode.ErrTrafficAgentMismatch
	}
	if stream != "" {
		var closed bool
		var owner string
		err = tx.QueryRowContext(ctx, `SELECT closed,node_id FROM xnode_traffic_streams WHERE agent_id=$1 AND stream_id=$2`, agent, stream).Scan(&closed, &owner)
		if err == nil {
			if owner != node {
				return xnode.ErrTrafficAgentMismatch
			}
			if closed {
				return xnode.ErrConflict // A replaced process may never reopen its stream.
			}
			return tx.Commit()
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	var old, owner string
	var sequence uint64
	err = tx.QueryRowContext(ctx, `SELECT stream_id,last_sequence,node_id FROM xnode_traffic_streams WHERE agent_id=$1 AND NOT closed FOR UPDATE`, agent).Scan(&old, &sequence, &owner)
	if err == nil {
		if owner != node {
			return xnode.ErrTrafficAgentMismatch
		}
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_traffic_streams SET closed=true,has_gaps=true,updated_at=$1 WHERE agent_id=$2 AND stream_id=$3`, stamp(time.Now().UTC()), agent, old); err != nil {
			return err
		}
		// The lost amount is unknown. Keep only the actual last committed cursor;
		// no synthetic sequence or usage is introduced during restart settlement.
		if err = recordDiscardedTrafficTx(ctx, tx, node, agent, sequence, sequence, "closed-"+old,
			"stream_closed_known_usage", "", false, wire.TrafficReport{StreamID: old}); err != nil {
			return err
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if stream != "" {
		now := stamp(time.Now().UTC())
		if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_traffic_streams(agent_id,stream_id,node_id,created_at,updated_at) VALUES($1,$2,$3,$4,$4)`, agent, stream, node, now); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_stream_rates(agent_id,stream_id,from_sequence,multiplier_fp) VALUES($1,$2,1,$3)`, agent, stream, xnode.MultiplierScale); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) XNodeStreamSequence(ctx context.Context, agent, stream string) (uint64, bool, error) {
	var sequence uint64
	var closed bool
	err := s.db.QueryRowContext(ctx, `SELECT last_sequence,closed FROM xnode_traffic_streams WHERE agent_id=$1 AND stream_id=$2`, agent, stream).Scan(&sequence, &closed)
	return sequence, closed, err
}

func lockXNodeStreamCursorTx(ctx context.Context, tx *sql.Tx, node, agent, stream string) (uint64, string, bool, error) {
	var sequence uint64
	var digest, owner string
	var closed bool
	err := tx.QueryRowContext(ctx, `SELECT last_sequence,last_digest,closed,node_id FROM xnode_traffic_streams WHERE agent_id=$1 AND stream_id=$2 FOR UPDATE`, agent, stream).Scan(&sequence, &digest, &closed, &owner)
	if err == nil && owner != node {
		err = xnode.ErrTrafficAgentMismatch
	}
	return sequence, digest, closed, err
}
