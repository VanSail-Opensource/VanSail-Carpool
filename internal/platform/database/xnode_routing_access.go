package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// PruneXNodeRoutingAccess removes operational choices under optimistic revision
// fencing. Existing compile-version triggers make every affected node receive
// the disabled graph through the normal master synchronization path, including
// reconnects. No upstream request or second delivery queue is created here.
func (s *Store) retryAttempt_PruneXNodeRoutingAccess(ctx context.Context, plan xnode.RoutingPlan, actor string, version int64, now time.Time) error {
	if actor == "" || plan.Cipher == "" {
		return xnode.ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE xnode_routing_plans SET cipher=$1,previous_cipher='',revision=revision+1,phase='disabled',reason='routing_access_removed',updated_at=$2 WHERE proxy_user_id=$3 AND revision=$4 AND operation_id=$5 AND phase=$6 AND (SELECT revision FROM xnode_compile_revision WHERE singleton=1)=$7 AND EXISTS(SELECT 1 FROM xnode_identities WHERE proxy_user_id=$8 AND user_id=$9)`, plan.Cipher, stamp(now), plan.ProxyUserID, plan.Revision, plan.OperationID, plan.Phase, version, plan.ProxyUserID, actor)
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
	// Converted customizations are superseded by the managed plan. Keep only
	// their historical encrypted evidence, not executable routing rows.
	if _, err := tx.ExecContext(ctx, `DELETE FROM xnode_customizations WHERE proxy_user_id=$1 AND user_id=$2`, plan.ProxyUserID, actor); err != nil {
		return err
	}
	if err := routingAuditTx(ctx, tx, actor, plan.OperationID, "access_pruned", "routing_access_removed", false, now); err != nil {
		return err
	}
	return tx.Commit()
}
