package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func lockNativeMemberIdentityTx(ctx context.Context, tx *sql.Tx, owner string, generation int64, exact bool, exclusive ...bool) error {
	var current int64
	var status string
	lock := " FOR SHARE"
	if len(exclusive) > 0 && exclusive[0] {
		lock = " FOR UPDATE"
	}
	if err := tx.QueryRowContext(ctx, `SELECT account_status,account_generation FROM users WHERE id=$1`+lock, owner).Scan(&status, &current); err != nil {
		return err
	}
	if status != "active" || exact && current != generation {
		return ErrAccountUnavailable
	}
	return nil
}

// Only the abandoned pre-association preparation from the older multi-write
// flow can be reused. Any publication, participant or accounting fact is retained
// unchanged and requires reconciliation instead of credential rotation.
func validateUnpublishedPAYGIdentityTx(ctx context.Context, tx *sql.Tx, identity xnode.Identity) error {
	if identity.UserID != "" || identity.PAYGOwnerID != "" || identity.HourlyOwnerID != "" || identity.FirstSyncedAt != nil || len(identity.SubscriptionHash) > 0 || identity.SubscriptionCipher != "" || identity.LegacyUserID != "" || identity.LifetimeBase != 0 {
		return fmt.Errorf("PAYG_PREPARATION_EXPOSURE_PENDING: %w", xnode.ErrAccessAccountingPending)
	}
	var exposed bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_payg_accounts WHERE proxy_user_id=$1)
 OR EXISTS(SELECT 1 FROM hourly_enrollments WHERE proxy_user_id=$1)
 OR EXISTS(SELECT 1 FROM xnode_service_bindings WHERE proxy_user_id=$1)
 OR EXISTS(SELECT 1 FROM xnode_periods p WHERE p.proxy_user_id=$1 AND (p.used_bytes<>0 OR p.purchase_id IS NOT NULL OR p.strategy<>'PAYG' OR p.status<>'open' OR EXISTS(SELECT 1 FROM xnode_period_nodes n WHERE n.period_id=p.id)))
 OR EXISTS(SELECT 1 FROM xnode_daily_usage WHERE proxy_user_id=$1)
 OR EXISTS(SELECT 1 FROM xnode_imported_usage WHERE proxy_user_id=$1)
 OR EXISTS(SELECT 1 FROM xnode_usage_minutes WHERE proxy_user_id=$1)`, identity.ProxyUserID).Scan(&exposed)
	if err != nil {
		return err
	}
	if exposed {
		return fmt.Errorf("PAYG_PREPARATION_ACCOUNTING_PENDING: %w", xnode.ErrAccessAccountingPending)
	}
	return nil
}
