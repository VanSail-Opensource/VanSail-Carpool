package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// EditAdminEntitlement atomically replaces mutable fields, audits, and queues sync.
func (s *Store) retryAttempt_EditAdminEntitlement(ctx context.Context, input AdminEntitlementEditInput, now time.Time) (model.Purchase, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Purchase{}, err
	}
	defer func() { _ = tx.Rollback() }()
	before, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=? AND purchases.user_id=?`, input.PurchaseID, input.UserID))
	if err != nil {
		return model.Purchase{}, err
	}
	before.SquadUUIDs, err = purchaseSquadsFrom(ctx, tx, before.ID)
	if err != nil {
		return model.Purchase{}, err
	}
	operation, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{
		ActorUserID: input.ActorUserID, OwnerUserID: input.UserID, Kind: providerops.KindAdminEntitlementEdit,
		IdempotencyKey: input.IdempotencyKey, RequestFingerprint: input.RequestFingerprint,
		Items: []providerops.ItemInput{{Key: input.UserID, TargetType: "user", TargetID: input.UserID}},
	}, now)
	if err != nil {
		return model.Purchase{}, err
	}
	if replayed {
		if err := tx.Commit(); err != nil {
			return model.Purchase{}, err
		}
		return s.PurchaseByID(ctx, input.PurchaseID)
	}
	if !before.UpdatedAt.Equal(input.ExpectedUpdatedAt) {
		return model.Purchase{}, ErrConflict
	}
	var comboExists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM combos WHERE id=?`, input.ComboID).Scan(&comboExists); err != nil {
		return model.Purchase{}, err
	}
	if comboExists != 1 {
		return model.Purchase{}, ErrNotFound
	}
	squads, err := json.Marshal(input.SquadUUIDs)
	if err != nil {
		return model.Purchase{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET combo_id=?,valid_from=?,valid_until=?,status=?,
		entitlement_traffic_limit_bytes=?,entitlement_reset_strategy=?,entitlement_squad_uuids=?,updated_at=?
		WHERE id=? AND user_id=? AND updated_at=?`, input.ComboID, stamp(input.ValidFrom), stamp(input.ValidUntil), input.Status,
		input.TrafficLimitBytes, input.ResetStrategy, string(squads), stamp(now), input.PurchaseID, input.UserID, stamp(before.UpdatedAt))
	if err != nil {
		return model.Purchase{}, fmt.Errorf("edit entitlement: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return model.Purchase{}, rowsErr
		}
		return model.Purchase{}, ErrConflict
	}
	if err := reconcilePurchaseContinuityTx(ctx, tx, input.PurchaseID, input.Status, input.ValidFrom, now); err != nil {
		return model.Purchase{}, err
	}
	if err := refreshPendingRolloverTx(ctx, tx, input.PurchaseID, input.TrafficLimitBytes, input.ComboID, now); err != nil {
		return model.Purchase{}, err
	}
	if err := insertEntitlementAudit(ctx, tx, input.ActorUserID, "entitlement.edit", input.PurchaseID,
		input.Reason, operation.Receipt.ID, entitlementSnapshot(before), map[string]any{"comboId": input.ComboID,
			"validFrom": input.ValidFrom, "validUntil": input.ValidUntil, "status": input.Status,
			"trafficLimitBytes": input.TrafficLimitBytes, "resetStrategy": input.ResetStrategy, "squadUuids": input.SquadUUIDs}, now); err != nil {
		return model.Purchase{}, err
	}
	var newComboName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM combos WHERE id=?`, input.ComboID).Scan(&newComboName); err != nil {
		return model.Purchase{}, err
	}
	kind, facts := adminEntitlementEditNotification(before, input, newComboName, now)
	if _, err := s.insertUserNotificationTx(ctx, tx, "admin:"+operation.Receipt.ID+":"+input.UserID, input.UserID,
		kind, providerItemGate(operation.Receipt.ID, input.UserID), facts, now); err != nil {
		return model.Purchase{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Purchase{}, err
	}
	return s.PurchaseByID(ctx, input.PurchaseID)
}

func refreshPendingRolloverTx(ctx context.Context, tx *sql.Tx, purchaseID string, trafficLimit int64, comboID string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE purchase_rollovers SET traffic_limit_bytes=?,
		minimum_remaining_bps=COALESCE((SELECT reward_rollover_min_remaining_bps FROM purchases WHERE id=?),
			(SELECT rollover_min_remaining_bps FROM combos WHERE id=?)),updated_at=?
		WHERE purchase_id=? AND status='pending'`, trafficLimit, purchaseID, comboID, stamp(now), purchaseID)
	return err
}

func entitlementSnapshot(purchase model.Purchase) map[string]any {
	return map[string]any{"comboId": purchase.ComboID, "validFrom": purchase.ValidFrom, "validUntil": purchase.ValidUntil,
		"status": purchase.Status, "trafficLimitBytes": purchase.TrafficLimitBytes,
		"resetStrategy": purchase.ResetStrategy, "squadUuids": purchase.SquadUUIDs}
}

func insertEntitlementAudit(ctx context.Context, tx *sql.Tx, actorID, action, purchaseID, reason, operationID string, before, after any, now time.Time) error {
	auditID, err := ids.New()
	if err != nil {
		return err
	}
	detail, err := json.Marshal(map[string]any{"reason": reason, "operationId": operationID, "before": before, "after": after})
	if err != nil {
		return fmt.Errorf("encode entitlement audit: %w", err)
	}
	return insertAuditTx(ctx, tx, auditID, &actorID, action, "purchase", purchaseID, string(detail), now)
}
