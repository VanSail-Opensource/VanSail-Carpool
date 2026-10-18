package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

type controlQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func comboControlConflictFrom(ctx context.Context, reader controlQueryer, userID string) (bool, error) {
	var conflict bool
	err := reader.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM provider_operations operation
 LEFT JOIN provider_operation_items item ON item.operation_id=operation.id
 WHERE operation.kind IN ('purchase_traffic_reset','purchase_refund','member_squad_switch','member_early_activation',
 'subscription_revoke','xnode_routing','admin_entitlement_edit','admin_entitlement_refund','admin_combo_replacement',
 'admin_bulk_extension','node_compensation','admin_temporary_ban','admin_temporary_unban')
 AND operation.status IN ('queued','processing','pending_review','partial') AND
 (operation.owner_user_id=? OR (item.target_type='user' AND item.target_id=?) OR
 (item.target_type='purchase' AND item.target_id IN (SELECT id FROM purchases WHERE user_id=?))))
 OR EXISTS(SELECT 1 FROM outbox_jobs job WHERE job.status='processing' AND
 ((json_extract(CASE WHEN json_valid(job.payload) THEN job.payload ELSE '{}' END,'$.userId')=? AND job.kind='remna_sync_user') OR
 (job.kind IN ('remna_apply_entitlement','remna_prepare_continuity','rollover_finalize') AND
 json_extract(CASE WHEN json_valid(job.payload) THEN job.payload ELSE '{}' END,'$.purchaseId') IN (SELECT id FROM purchases WHERE user_id=?))))
 OR EXISTS(SELECT 1 FROM admin_temporary_bans WHERE user_id=? AND restored_at IS NULL)`, userID, userID, userID, userID, userID, userID).Scan(&conflict)
	return conflict, err
}

// ComboControlConflict prevents switching while another provider mutation owns the member.
func (s *Store) ComboControlConflict(ctx context.Context, userID string) (bool, error) {
	return comboControlConflictFrom(ctx, s.db, userID)
}

func activeControlPurchaseTx(ctx context.Context, tx *sql.Tx, userID, purchaseID string, now time.Time) (model.Purchase, error) {
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=? AND purchases.user_id=?`, purchaseID, userID))
	if err != nil {
		return model.Purchase{}, err
	}
	if purchase.Status != "active" || now.Before(purchase.ValidFrom) || !now.Before(purchase.ValidUntil) {
		return model.Purchase{}, purchaseops.ErrIneligible
	}
	var other bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=? AND id<>? AND status IN ('active','activating') AND valid_until>?)`, userID, purchaseID, stamp(now)).Scan(&other)
	if err != nil {
		return model.Purchase{}, err
	}
	if other {
		return model.Purchase{}, ErrConflict
	}
	return purchase, nil
}

func auditComboControlTx(ctx context.Context, tx *sql.Tx, userID, action, purchaseID string, details map[string]any, now time.Time) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	body, err := json.Marshal(details)
	if err != nil {
		return err
	}
	return insertAuditTx(ctx, tx, id, &userID, action, "purchase", purchaseID, string(body), now)
}

func queuedControlPurchaseTx(ctx context.Context, tx *sql.Tx, userID, queuedID string) (model.Purchase, error) {
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.user_id=? AND purchases.status='queued'
 ORDER BY purchases.valid_from,purchases.created_at,purchases.id LIMIT 1`, userID))
	if errors.Is(err, ErrNotFound) {
		return model.Purchase{}, purchaseops.ErrIneligible
	}
	if err != nil {
		return model.Purchase{}, err
	}
	if purchase.ID != queuedID || !purchase.ValidUntil.After(purchase.ValidFrom) {
		return model.Purchase{}, purchaseops.ErrIneligible
	}
	return purchase, nil
}
