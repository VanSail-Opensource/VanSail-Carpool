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
func (s *Store) PruneXNodeRoutingAccess(ctx context.Context, plan xnode.RoutingPlan, actor string, version int64, now time.Time) error {
	if actor == "" || plan.Cipher == "" {
		return xnode.ErrInvalid
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE xnode_routing_plans SET cipher=?,previous_cipher='',revision=revision+1,phase='disabled',reason='routing_access_removed',updated_at=? WHERE proxy_user_id=? AND revision=? AND operation_id=? AND phase=? AND (SELECT revision FROM xnode_compile_revision WHERE singleton=1)=? AND EXISTS(SELECT 1 FROM xnode_identities WHERE proxy_user_id=? AND user_id=?)`, plan.Cipher, stamp(now), plan.ProxyUserID, plan.Revision, plan.OperationID, plan.Phase, version, plan.ProxyUserID, actor)
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
	if _, err := tx.ExecContext(ctx, `DELETE FROM xnode_customizations WHERE proxy_user_id=? AND user_id=?`, plan.ProxyUserID, actor); err != nil {
		return err
	}
	if err := routingAuditTx(ctx, tx, actor, plan.OperationID, "access_pruned", "routing_access_removed", false, now); err != nil {
		return err
	}
	return tx.Commit()
}
