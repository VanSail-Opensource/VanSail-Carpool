package database

import (
	"context"
	"database/sql"
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Store) FinalizeAccountNativePurgePeriod(ctx context.Context, target model.AccountPurge, jobID string, period xnode.Period) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	_, allowed, err := s.accountNativePurgeGuardTx(ctx, tx, target, jobID)
	if err != nil || !allowed {
		return false, err
	}
	if err = accountPurgeClosingPeriodTx(ctx, tx, target, period); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE xnode_periods SET status='closed' WHERE id=$1 AND status='closing'
 AND NOT EXISTS(SELECT 1 FROM xnode_period_nodes WHERE period_id=$1 AND flushed_sequence IS NULL)`, period.ID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if count != 1 {
		return false, xnode.ErrAccessAccountingPending
	}
	return true, tx.Commit()
}

func accountPurgeClosingPeriodTx(ctx context.Context, tx *sql.Tx, target model.AccountPurge, expected xnode.Period) error {
	if expected.EndedAt == nil || expected.Status != "closing" {
		return xnode.ErrInvalid
	}
	period, err := scanXNodePeriod(tx.QueryRowContext(ctx, `SELECT `+xnodePeriodColumns+` FROM xnode_periods p
 WHERE p.id=$3 AND `+accountNativePeriodScope+` FOR UPDATE`, target.UserID, target.ThroughGeneration, expected.ID))
	if err != nil {
		return err
	}
	if period.ProxyUserID != expected.ProxyUserID || period.PurchaseID != expected.PurchaseID || period.Status != "closing" || period.ClosureGeneration != expected.ClosureGeneration || period.EndedAt == nil || !period.EndedAt.Equal(*expected.EndedAt) {
		return xnode.ErrConflict
	}
	return nil
}

// The native barrier is immutable accounting evidence, but the cleanup phase
// still serializes it with a newer join/admin promotion before mutating state.
func (s *Store) SealAccountNativePurgePeriodNode(ctx context.Context, target model.AccountPurge, jobID string, period xnode.Period, node string, barrier wire.TrafficBarrier) (bool, error) {
	if !barrier.Quiesced || barrier.AccountingPeriod != period.ID || barrier.Sequence > math.MaxInt64 || barrier.LegacySequence > math.MaxInt64 || barrier.StreamID != "" && !wire.ValidStreamID(barrier.StreamID) {
		return false, xnode.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	_, allowed, err := s.accountNativePurgeGuardTx(ctx, tx, target, jobID)
	if err != nil || !allowed {
		return false, err
	}
	if err = accountPurgeClosingPeriodTx(ctx, tx, target, period); err != nil {
		return false, err
	}
	var agent string
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(agent_id,'') FROM xnode_nodes WHERE id=$1 FOR SHARE`, node).Scan(&agent); err != nil {
		return false, err
	}
	if agent == "" || agent != barrier.AgentID {
		return false, xnode.ErrConflict
	}
	sequence, err := committedPeriodBarrierTx(ctx, tx, node, barrier)
	if err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE xnode_period_nodes SET flushed_sequence=$1,flushed_stream_id=$2,
 flushed_legacy_sequence=$3 WHERE period_id=$4 AND node_id=$5 AND flushed_sequence IS NULL`, sequence, barrier.StreamID, barrier.LegacySequence, period.ID, node)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if count != 1 {
		var sealed bool
		if err = tx.QueryRowContext(ctx, `SELECT flushed_sequence IS NOT NULL FROM xnode_period_nodes WHERE period_id=$1 AND node_id=$2`, period.ID, node).Scan(&sealed); err != nil {
			return false, err
		}
		if !sealed {
			return false, xnode.ErrConflict
		}
	}
	return true, tx.Commit()
}
