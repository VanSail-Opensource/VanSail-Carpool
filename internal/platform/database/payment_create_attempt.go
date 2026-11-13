package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type paymentCreateAdmission struct {
	item    providerops.Item
	order   model.PaymentOrder
	allowed bool
}

// BeginPaymentCreateAttempt binds the external attempt marker to the current
// payable order and owner lifecycle. Cancellation and expiry cannot win before
// the marker yet leave a stale worker authorized to create another checkout.
func (s *Store) BeginPaymentCreateAttempt(ctx context.Context, operationID, itemKey string, now time.Time) (providerops.Item, model.PaymentOrder, bool, error) {
	started := time.Now()
	admission, err := hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (paymentCreateAdmission, error) {
		operation, err := scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=$1 FOR UPDATE`, operationID))
		if err != nil {
			return paymentCreateAdmission{}, err
		}
		if operation.Receipt.Kind != "payment_create" || operation.Receipt.Status != string(providerops.StatusProcessing) ||
			operation.AttemptStartedAt == nil || operation.OwnerUserID == "" || operation.ActorUserID != operation.OwnerUserID {
			return paymentCreateAdmission{}, ErrConflict
		}
		item, err := scanProviderOperationItem(tx.QueryRowContext(ctx, providerOperationItemSelect+` WHERE operation_id=$1 AND item_key=$2 FOR UPDATE`, operationID, itemKey))
		if err != nil {
			return paymentCreateAdmission{}, err
		}
		if item.TargetType != "payment_order" || item.TargetID == "" {
			return paymentCreateAdmission{}, ErrConflict
		}
		order, err := scanPaymentOrder(tx.QueryRowContext(ctx, paymentSelect+` WHERE id=$1 FOR UPDATE`, item.TargetID))
		if err != nil {
			return paymentCreateAdmission{}, err
		}
		result := paymentCreateAdmission{item: item, order: order}
		if order.UserID != operation.OwnerUserID {
			return paymentCreateAdmission{}, ErrConflict
		}
		if providerops.Terminal(item.Status) || order.Status != "creating" || order.CancelledAt != nil || order.PaidAt != nil {
			return result, nil
		}
		if err := lockProviderOperationAccountsTx(ctx, tx, operation.ActorUserID, operation.OwnerUserID); err != nil {
			return paymentCreateAdmission{}, err
		}
		// Preserve the supplied clock while charging real lock/retry waiting time.
		at := now.UTC().Add(time.Since(started))
		if !at.Before(order.ExpiresAt) {
			if _, err := tx.ExecContext(ctx, `UPDATE payment_orders SET status='expired',provider_payload='{}',payment_url=NULL,
				qr_payload=CASE WHEN receiving_address IS NOT NULL THEN NULL ELSE qr_payload END,updated_at=$1 WHERE id=$2`, stamp(at), order.ID); err != nil {
				return paymentCreateAdmission{}, err
			}
			result.order.Status, result.order.UpdatedAt = "expired", at
			result.order.PaymentURL = nil
			if result.order.ReceivingAddress != nil {
				result.order.QRPayload = nil
			}
			return result, nil
		}
		if item.Status == providerops.StatusProcessing {
			// Only the built-in reservation can be replayed without another remote
			// mutation. Its own transaction retains the original quote identity.
			result.allowed = order.Provider == "builtin_usdt" && item.AttemptStartedAt != nil
			return result, nil
		}
		if item.Status != providerops.StatusQueued {
			return paymentCreateAdmission{}, ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE provider_operation_items SET status='processing',attempt_started_at=$1,updated_at=$1
			WHERE operation_id=$2 AND item_key=$3`, stamp(at), operationID, itemKey); err != nil {
			return paymentCreateAdmission{}, err
		}
		result.item.Status, result.item.AttemptStartedAt, result.item.UpdatedAt = providerops.StatusProcessing, &at, at
		result.allowed = true
		return result, nil
	})
	return admission.item, admission.order, admission.allowed, err
}
