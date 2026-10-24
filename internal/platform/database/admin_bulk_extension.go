package database

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// CreateAdminBulkExtension atomically shifts targets and queues exact provider sync.
func (s *Store) retryAttempt_CreateAdminBulkExtension(ctx context.Context, input AdminBulkExtensionInput, now time.Time) (model.OperationReceipt, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	defer func() { _ = tx.Rollback() }()
	targets, preview, err := adminBulkTargets(ctx, tx, input.Filter, now)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if len(targets) == 0 {
		return model.OperationReceipt{}, ErrConflict
	}
	items := make([]providerops.ItemInput, 0, len(targets))
	for _, target := range targets {
		items = append(items, providerops.ItemInput{Key: target.UserID, TargetType: "user", TargetID: target.UserID})
	}
	operation, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{
		ActorUserID: input.ActorUserID, Kind: providerops.KindAdminBulkExtension,
		IdempotencyKey: input.IdempotencyKey, RequestFingerprint: input.RequestFingerprint, Items: items,
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
	for _, target := range targets {
		before, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=?`, target.PurchaseID))
		if err != nil {
			return model.OperationReceipt{}, err
		}
		if err := shiftAdminSubscriptionTx(ctx, tx, target, input.DurationMinutes, now); err != nil {
			return model.OperationReceipt{}, err
		}
		newExpiry, err := addSubscriptionMinutes(before.ValidUntil, input.DurationMinutes)
		if err != nil {
			return model.OperationReceipt{}, err
		}
		facts := adminNotificationBase("entitlement_edit", input.Reason, now)
		facts[notifications.FactAddedSeconds] = strconv.FormatInt(int64(newExpiry.Sub(before.ValidUntil)/time.Second), 10)
		facts[notifications.FactPreviousExpiry] = before.ValidUntil.Format(time.RFC3339Nano)
		facts[notifications.FactNewExpiry] = newExpiry.Format(time.RFC3339Nano)
		facts[notifications.FactCombo] = before.ComboName
		if _, err := s.insertUserNotificationTx(ctx, tx, "admin:"+operation.Receipt.ID+":"+target.UserID, target.UserID,
			jobpayload.UserEventAdminExtension, providerItemGate(operation.Receipt.ID, target.UserID), facts, now); err != nil {
			return model.OperationReceipt{}, err
		}
	}
	auditID, err := ids.New()
	if err != nil {
		return model.OperationReceipt{}, err
	}
	detail, err := json.Marshal(map[string]any{"reason": input.Reason, "operationId": operation.Receipt.ID,
		"comboIds": input.Filter.ComboIDs, "addonSquadUuids": input.Filter.AddonSquadUUIDs,
		"durationMinutes": input.DurationMinutes, "matchedUsers": preview.MatchedUsers, "queuedSuccessors": preview.QueuedSuccessors})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if err := insertAuditTx(ctx, tx, auditID, &input.ActorUserID, "entitlement.bulk_extend", "provider_operation",
		operation.Receipt.ID, string(detail), now); err != nil {
		return model.OperationReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.OperationReceipt{}, err
	}
	return operation.Receipt, nil
}
