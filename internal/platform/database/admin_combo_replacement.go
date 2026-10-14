package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// ReplaceAdminCombo changes active configuration without moving TXB facts.
func (s *Store) ReplaceAdminCombo(ctx context.Context, input AdminComboReplacementInput, now time.Time) (model.OperationReceipt, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	defer func() { _ = tx.Rollback() }()
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.user_id=?
		AND purchases.status IN ('activating','active') AND purchases.valid_from<=? AND purchases.valid_until>?
		ORDER BY purchases.valid_from DESC LIMIT 1`, input.UserID, stamp(now), stamp(now)))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	purchase.SquadUUIDs, err = purchaseSquadsFrom(ctx, tx, purchase.ID)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{
		ActorUserID: input.ActorUserID, OwnerUserID: input.UserID, Kind: providerops.KindAdminComboReplacement,
		IdempotencyKey: input.IdempotencyKey, RequestFingerprint: input.RequestFingerprint,
		Items: []providerops.ItemInput{{Key: input.UserID, TargetType: "user", TargetID: input.UserID}},
	}, now)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if replayed {
		if err := tx.Commit(); err != nil {
			return model.OperationReceipt{}, err
		}
		return operation.Receipt, nil
	}
	var comboExists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM combos WHERE id=?`, input.ComboID).Scan(&comboExists); err != nil {
		return model.OperationReceipt{}, err
	}
	if comboExists != 1 {
		return model.OperationReceipt{}, ErrNotFound
	}
	addons, err := json.Marshal(input.AddonSquadUUIDs)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET combo_id=?,entitlement_traffic_limit_bytes=NULL,
		entitlement_reset_strategy=NULL,entitlement_squad_uuids=NULL,entitlement_addon_squad_uuids=?,
		reward_renewal_price_minor=NULL,reward_rollover_min_remaining_bps=NULL,reward_traffic_renewal=1,
		reward_renewal_traffic_limit_bytes=NULL,updated_at=?
		WHERE id=? AND user_id=? AND status IN ('activating','active')`, input.ComboID, string(addons), stamp(now), purchase.ID, input.UserID)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return model.OperationReceipt{}, rowsErr
		}
		return model.OperationReceipt{}, ErrConflict
	}
	if err := insertEntitlementAudit(ctx, tx, input.ActorUserID, "entitlement.combo_replace", purchase.ID, input.Reason,
		operation.Receipt.ID, entitlementSnapshot(purchase), map[string]any{"comboId": input.ComboID,
			"addonSquadUuids": input.AddonSquadUUIDs, "txbMovementMinor": 0}, now); err != nil {
		return model.OperationReceipt{}, err
	}
	var newComboName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM combos WHERE id=?`, input.ComboID).Scan(&newComboName); err != nil {
		return model.OperationReceipt{}, err
	}
	facts := adminNotificationBase("combo_replacement", input.Reason, now)
	facts[notifications.FactPreviousCombo], facts[notifications.FactNewCombo] = purchase.ComboName, newComboName
	if len(input.AddonSquadUUIDs) > 0 {
		facts[notifications.FactAddOns] = squadSummary(input.AddonSquadUUIDs)
	}
	if _, err := s.insertUserNotificationTx(ctx, tx, "admin:"+operation.Receipt.ID+":"+input.UserID, input.UserID,
		jobpayload.UserEventAdminUpdate, providerItemGate(operation.Receipt.ID, input.UserID), facts, now); err != nil {
		return model.OperationReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.OperationReceipt{}, err
	}
	return operation.Receipt, nil
}
