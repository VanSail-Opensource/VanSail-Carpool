package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type trafficCursorKey struct{ agent, stream string }
type trafficNodeValue struct {
	agent string
}
type trafficCursorValue struct {
	node, digest        string
	sequence            uint64
	closed, gaps, dirty bool
}
type trafficBatchState struct {
	nodes   map[string]trafficNodeValue
	cursors map[trafficCursorKey]*trafficCursorValue
	payg    map[int64]bool
	native  *nativeTrafficBuffer
}

func trafficNodeTx(ctx context.Context, tx *sql.Tx, batch *trafficBatchState, node string) (string, error) {
	if batch != nil {
		v, exists := batch.nodes[node]
		if !exists {
			return "", xnode.ErrNotFound
		}
		return v.agent, nil
	}
	var agent string
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(agent_id,'') FROM xnode_nodes WHERE id=$1 FOR KEY SHARE`, node).Scan(&agent)
	return agent, err
}

func trafficCursorTx(ctx context.Context, tx *sql.Tx, batch *trafficBatchState, node string, key trafficCursorKey) (*trafficCursorValue, error) {
	if batch != nil {
		v := batch.cursors[key]
		if v == nil || v.node != node {
			return nil, xnode.ErrTrafficAgentMismatch
		}
		return v, nil
	}
	v := &trafficCursorValue{node: node}
	var err error
	if key.stream == "" {
		v.sequence, v.digest, err = lockXNodeCursorTx(ctx, tx, node, key.agent)
	} else {
		v.sequence, v.digest, v.closed, err = lockXNodeStreamCursorTx(ctx, tx, node, key.agent, key.stream)
	}
	return v, err
}

func storeTrafficCursorTx(ctx context.Context, tx *sql.Tx, batch *trafficBatchState, node string, key trafficCursorKey, sequence uint64, digest string, gaps bool) error {
	if batch != nil {
		v := batch.cursors[key]
		v.sequence, v.digest, v.gaps, v.dirty = sequence, digest, v.gaps || gaps, true
		return nil
	}
	var err error
	if key.stream == "" {
		_, err = tx.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(agent_id) DO UPDATE SET last_sequence=excluded.last_sequence,last_digest=excluded.last_digest,updated_at=excluded.updated_at`, key.agent, node, sequence, digest, stamp(time.Now().UTC()))
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE xnode_traffic_streams SET last_sequence=$1,last_digest=$2,has_gaps=has_gaps OR $3,updated_at=$4 WHERE agent_id=$5 AND stream_id=$6`, sequence, digest, gaps, stamp(time.Now().UTC()), key.agent, key.stream)
	}
	return err
}
