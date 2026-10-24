package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"strconv"
)

// RefundAdminEntitlement atomically credits one cancellation and queues exact sync.
func (s *Store) retryAttempt_RefundAdminEntitlement(ctx context.Context, input AdminEntitlementRefundInput, now time.Time) (model.OperationReceipt, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	defer func() { _ = tx.Rollback() }()
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=? AND purchases.user_id=?`, input.PurchaseID, input.UserID))
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, replayed, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{
		ActorUserID: input.ActorUserID, OwnerUserID: input.UserID, Kind: providerops.KindAdminEntitlementRefund,
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
	if purchase.Status == "cancelled" || purchase.Status == "expired" || purchase.Status == "failed" {
		return model.OperationReceipt{}, ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET status='cancelled',updated_at=?
		WHERE id=? AND user_id=? AND status IN ('activating','active','queued')`, stamp(now), input.PurchaseID, input.UserID)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return model.OperationReceipt{}, rowsErr
		}
		return model.OperationReceipt{}, ErrConflict
	}
	if input.AmountTXBMinor <= 0 || input.AmountTXBMinor > purchase.PriceTXBMinor {
		return model.OperationReceipt{}, ErrConflict
	}
	balance, err := adjustBalanceTx(ctx, tx, input.UserID, input.AmountTXBMinor, now)
	if err != nil {
		return model.OperationReceipt{}, fmt.Errorf("credit entitlement refund: %w", err)
	}
	if _, err := insertLedgerTx(ctx, tx, input.UserID, input.AmountTXBMinor, balance,
		"admin_entitlement_refund", purchase.ID, input.Reason, now); err != nil {
		return model.OperationReceipt{}, err
	}
	if err := insertEntitlementAudit(ctx, tx, input.ActorUserID, "entitlement.refund", purchase.ID, input.Reason,
		operation.Receipt.ID, entitlementSnapshot(purchase), map[string]any{"status": "cancelled", "creditedTxbMinor": input.AmountTXBMinor}, now); err != nil {
		return model.OperationReceipt{}, err
	}
	facts := adminNotificationBase("entitlement_refund", input.Reason, now)
	facts[notifications.FactCombo] = purchase.ComboName
	facts[notifications.FactCredited] = strconv.FormatInt(input.AmountTXBMinor, 10)
	facts[notifications.FactBalance] = strconv.FormatInt(balance, 10)
	facts[notifications.FactPreviousStatus], facts[notifications.FactNewStatus] = purchase.Status, "cancelled"
	if _, err := s.insertUserNotificationTx(ctx, tx, "admin:"+operation.Receipt.ID+":"+input.UserID, input.UserID,
		jobpayload.UserEventAdminUpdate, providerItemGate(operation.Receipt.ID, input.UserID), facts, now); err != nil {
		return model.OperationReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.OperationReceipt{}, err
	}
	return operation.Receipt, nil
}
