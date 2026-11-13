package database

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// FinalizeXNodePeriod seals only the closure whose participant barriers were
// observed. Reopening or a newer closure boundary fences a stale network wait.
func (s *Store) FinalizeXNodePeriod(ctx context.Context, period xnode.Period) error {
	if period.EndedAt == nil {
		return xnode.ErrConflict
	}
	result, err := s.db.ExecContext(ctx, `UPDATE xnode_periods SET status='closed' WHERE id=$1 AND proxy_user_id=$2 AND purchase_id IS NOT DISTINCT FROM $3 AND status='closing' AND `+purchaseTimestampOrderSQL("ended_at")+` = $4 AND closure_generation=$5 AND NOT EXISTS(SELECT 1 FROM xnode_period_nodes WHERE period_id=$1 AND flushed_sequence IS NULL)`, period.ID, period.ProxyUserID, nullText(period.PurchaseID), purchaseTimestampOrder(*period.EndedAt), period.ClosureGeneration)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return xnode.ErrConflict
	}
	return nil
}
