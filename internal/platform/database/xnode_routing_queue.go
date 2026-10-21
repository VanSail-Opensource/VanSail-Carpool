package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

// queueXNodeRoutingTx shares desired graph/audit writes with atomic DIY saves.
func queueXNodeRoutingTx(ctx context.Context, tx *sql.Tx, p xnode.RoutingPlan, input providerops.CreateInput, operation string, now time.Time) error {
	old, err := scanRoutingPlan(tx.QueryRowContext(ctx, routingPlanSelect+` WHERE proxy_user_id=?`, p.ProxyUserID))
	if err != nil && !errors.Is(err, xnode.ErrNotFound) {
		return err
	}
	if old.Revision != p.Revision || old.Phase != "" && old.Phase != "active" && old.Phase != "disabled" {
		return xnode.ErrConflict
	}
	phase, reason := "revoking", ""
	if p.Phase == "disabled" {
		phase, reason = "disabled", p.Reason
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_routing_plans(proxy_user_id,cipher,previous_cipher,legacy_cipher,revision,phase,operation_id,reason,updated_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(proxy_user_id) DO UPDATE SET cipher=excluded.cipher,previous_cipher=excluded.previous_cipher,legacy_cipher=excluded.legacy_cipher,revision=excluded.revision,phase=excluded.phase,operation_id=excluded.operation_id,reason=excluded.reason,updated_at=excluded.updated_at`, p.ProxyUserID, p.Cipher, old.Cipher, p.LegacyCipher, p.Revision+1, phase, operation, reason, stamp(now))
	if err != nil {
		return err
	}
	if err = routingAuditTx(ctx, tx, input.ActorUserID, operation, phase, reason, p.LegacyCipher != "", now); err != nil {
		return err
	}
	return nil
}
