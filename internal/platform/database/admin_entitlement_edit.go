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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Purchase{}, err
	}
	defer func() { _ = tx.Rollback() }()
	before, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=$1 AND purchases.user_id=$2`, input.PurchaseID, input.UserID))
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
		return before, nil
	}
	// Refunding a term settles its original debit. Reopening or rewriting that
	// same row could make a different refund lane credit the same debit again.
	// Retained receipts stay readable/replayable, while a new grant gets a new ID.
	refunded, err := entitlementRefundedTx(ctx, tx, before)
	if err != nil {
		return model.Purchase{}, err
	}
	if refunded {
		return model.Purchase{}, fmt.Errorf("refunded entitlement is immutable: %w", ErrConflict)
	}
	if !before.UpdatedAt.Equal(input.ExpectedUpdatedAt) {
		return model.Purchase{}, ErrConflict
	}
	var comboExists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM combos WHERE id=$1`, input.ComboID).Scan(&comboExists); err != nil {
		return model.Purchase{}, err
	}
	if comboExists != 1 {
		return model.Purchase{}, ErrNotFound
	}
	squads, err := json.Marshal(input.SquadUUIDs)
	if err != nil {
		return model.Purchase{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET combo_id=$1,valid_from=$2,valid_until=$3,status=$4,
		entitlement_traffic_limit_bytes=$5,entitlement_reset_strategy=$6,entitlement_squad_uuids=$7,updated_at=$8
		WHERE id=$9 AND user_id=$10 AND updated_at=$11`, input.ComboID, stamp(input.ValidFrom), stamp(input.ValidUntil), input.Status,
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
	if err := retirePurchaseContinuityTx(ctx, tx, input.PurchaseID, now); err != nil {
		return model.Purchase{}, err
	}
	if err := insertEntitlementAudit(ctx, tx, input.ActorUserID, "entitlement.edit", input.PurchaseID,
		input.Reason, operation.Receipt.ID, entitlementSnapshot(before), map[string]any{"comboId": input.ComboID,
			"validFrom": input.ValidFrom, "validUntil": input.ValidUntil, "status": input.Status,
			"trafficLimitBytes": input.TrafficLimitBytes, "resetStrategy": input.ResetStrategy, "squadUuids": input.SquadUUIDs}, now); err != nil {
		return model.Purchase{}, err
	}
	var newComboName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM combos WHERE id=$1`, input.ComboID).Scan(&newComboName); err != nil {
		return model.Purchase{}, err
	}
	kind, facts := adminEntitlementEditNotification(before, input, newComboName, now)
	if _, err := s.insertUserNotificationTx(ctx, tx, "admin:"+operation.Receipt.ID+":"+input.UserID, input.UserID,
		kind, providerItemGate(operation.Receipt.ID, input.UserID), facts, now); err != nil {
		return model.Purchase{}, err
	}
	after, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=$1 AND purchases.user_id=$2`, input.PurchaseID, input.UserID))
	if err != nil {
		return model.Purchase{}, err
	}
	after.SquadUUIDs, err = purchaseSquadsFrom(ctx, tx, after.ID)
	if err != nil {
		return model.Purchase{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Purchase{}, err
	}
	return after, nil
}

func entitlementRefundedTx(ctx context.Context, tx *sql.Tx, purchase model.Purchase) (bool, error) {
	var refunded bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ledger_entries entry WHERE entry.user_id=$1 AND (
		(entry.reference_id=$2 AND entry.kind IN ('admin_entitlement_refund','admin_entitlement_cancellation','purchase_cancellation'))
		OR (entry.kind='member_refund_credit' AND EXISTS(SELECT 1 FROM provider_operation_items item
			WHERE item.operation_id=entry.reference_id AND item.target_type='purchase' AND item.target_id=$2))))`,
		purchase.UserID, purchase.ID).Scan(&refunded)
	return refunded, err
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
