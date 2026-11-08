package database

import (
	"context"
	"database/sql"
	"errors"
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type periodSealScope int

const (
	closingPeriodSeal periodSealScope = iota
	removingNodeSeal
)

// SealXNodeRemovalPeriodNode retains a participant barrier only after permanent
// node revocation. The member's open quota period need not itself be closing.
func (s *Store) SealXNodeRemovalPeriodNode(ctx context.Context, expected xnode.Period, node string, barrier wire.TrafficBarrier) error {
	return s.sealPeriodNode(ctx, expected, node, barrier, removingNodeSeal)
}

func (s *Store) sealPeriodNode(ctx context.Context, expected xnode.Period, node string, barrier wire.TrafficBarrier, scope periodSealScope) error {
	if !barrier.Quiesced || barrier.AccountingPeriod != expected.ID || barrier.Sequence > math.MaxInt64 || barrier.LegacySequence > math.MaxInt64 || barrier.StreamID != "" && !wire.ValidStreamID(barrier.StreamID) {
		return xnode.ErrInvalid
	}
	if scope == closingPeriodSeal && (expected.Status != "closing" || expected.ClosureGeneration < 0) {
		return xnode.ErrInvalid
	}
	for retry := 0; ; retry++ {
		err := s.sealPeriodNodeAttempt(ctx, expected, node, barrier, scope)
		if !s.retryTransaction(ctx, err, retry) {
			return err
		}
	}
}

func (s *Store) sealPeriodNodeAttempt(ctx context.Context, expected xnode.Period, node string, barrier wire.TrafficBarrier, scope periodSealScope) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var generation int64
	var status string
	err = tx.QueryRowContext(ctx, `SELECT closure_generation,status FROM xnode_periods WHERE id=$1 AND proxy_user_id=$2 AND purchase_id IS NOT DISTINCT FROM $3 FOR UPDATE`, expected.ID, expected.ProxyUserID, nullText(expected.PurchaseID)).Scan(&generation, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return xnode.ErrConflict
	}
	if err != nil {
		return err
	}
	if scope == closingPeriodSeal && (generation != expected.ClosureGeneration || status != "closing") {
		return xnode.ErrConflict
	}
	var agent string
	var fenced bool
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(agent_id,''),enabled=0 AND (removing_at IS NOT NULL OR retired_at IS NOT NULL) FROM xnode_nodes WHERE id=$1 FOR SHARE`, node).Scan(&agent, &fenced)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (agent == "" || agent != barrier.AgentID || scope == removingNodeSeal && !fenced) {
		return xnode.ErrConflict
	}
	if err != nil {
		return err
	}
	sequence, err := committedPeriodBarrierTx(ctx, tx, node, barrier)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE xnode_period_nodes SET flushed_sequence=$1,flushed_stream_id=$2,flushed_legacy_sequence=$3 WHERE period_id=$4 AND node_id=$5 AND flushed_sequence IS NULL`, sequence, barrier.StreamID, barrier.LegacySequence, expected.ID, node)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		var sealed bool
		if err = tx.QueryRowContext(ctx, `SELECT flushed_sequence IS NOT NULL FROM xnode_period_nodes WHERE period_id=$1 AND node_id=$2`, expected.ID, node).Scan(&sealed); errors.Is(err, sql.ErrNoRows) || err == nil && !sealed {
			return xnode.ErrConflict
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func committedPeriodBarrierTx(ctx context.Context, tx *sql.Tx, node string, barrier wire.TrafficBarrier) (uint64, error) {
	var legacy uint64
	var legacyNode string
	err := tx.QueryRowContext(ctx, `SELECT node_id,last_sequence FROM xnode_report_cursors WHERE agent_id=$1`, barrier.AgentID).Scan(&legacyNode, &legacy)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if err == nil && legacyNode != node {
		return 0, xnode.ErrConflict
	}
	if barrier.StreamID == "" {
		if legacy < barrier.Sequence {
			return 0, xnode.ErrConflict
		}
		return barrier.Sequence, nil
	}
	if legacy < barrier.LegacySequence {
		return 0, xnode.ErrConflict
	}
	var sequence uint64
	var closed bool
	err = tx.QueryRowContext(ctx, `SELECT last_sequence,closed FROM xnode_traffic_streams WHERE agent_id=$1 AND stream_id=$2 AND node_id=$3`, barrier.AgentID, barrier.StreamID, node).Scan(&sequence, &closed)
	if err != nil {
		return 0, err
	}
	if !closed && sequence < barrier.Sequence {
		return 0, xnode.ErrConflict
	}
	return min(sequence, barrier.Sequence), nil
}
