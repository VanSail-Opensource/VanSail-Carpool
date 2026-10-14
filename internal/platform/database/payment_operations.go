package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// BeginPaymentOrderOperation atomically stores a priced order and queues checkout creation.
func (s *Store) BeginPaymentOrderOperation(ctx context.Context, order model.PaymentOrder,
	input providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, fmt.Errorf("begin payment create command: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	operation, replayed, err := createProviderOperationTx(ctx, tx, input, now.UTC())
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if !replayed {
		if err := expireStalePaymentsTx(ctx, tx, now.UTC()); err != nil {
			return providerops.Operation{}, false, err
		}
		if err := insertPaymentOrderTx(ctx, tx, order, now.UTC()); err != nil {
			return providerops.Operation{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return providerops.Operation{}, false, fmt.Errorf("commit payment create command: %w", err)
	}
	return operation, replayed, nil
}

// BeginPaymentCancellationOperation atomically stops polling and queues provider cancellation.
func (s *Store) BeginPaymentCancellationOperation(ctx context.Context, orderID, userID string,
	input providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, fmt.Errorf("begin payment cancel command: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	operation, replayed, err := createProviderOperationTx(ctx, tx, input, now.UTC())
	if err != nil || replayed {
		return commitPaymentOperationReplay(tx, operation, replayed, err)
	}
	order, err := scanPaymentOrder(tx.QueryRowContext(ctx, paymentSelect+` WHERE id=? AND user_id=?`, orderID, userID))
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if (order.Status != "creating" && order.Status != "pending") || order.CancelledAt != nil {
		return providerops.Operation{}, false, ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE payment_orders SET cancelled_at=?,cancel_reason='cancelled by user',
		provider_payload='{}',payment_url=NULL,qr_payload=CASE WHEN receiving_address IS NOT NULL THEN NULL ELSE qr_payload END,
		updated_at=? WHERE id=? AND user_id=? AND status IN ('creating','pending') AND paid_at IS NULL AND cancelled_at IS NULL`,
		stamp(now), stamp(now), orderID, userID)
	if err != nil {
		return providerops.Operation{}, false, fmt.Errorf("cancel payment for provider command: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return providerops.Operation{}, false, rowsErr
		}
		return providerops.Operation{}, false, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return providerops.Operation{}, false, fmt.Errorf("commit payment cancel command: %w", err)
	}
	return operation, false, nil
}

func commitPaymentOperationReplay(tx *sql.Tx, operation providerops.Operation, replayed bool,
	err error) (providerops.Operation, bool, error) {
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return providerops.Operation{}, false, err
	}
	return operation, replayed, nil
}

func expireStalePaymentsTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE payment_orders SET status='expired',provider_payload='{}',payment_url=NULL,
		qr_payload=CASE WHEN receiving_address IS NOT NULL THEN NULL ELSE qr_payload END,updated_at=?
		WHERE provider<>'bepusdt' AND status IN ('creating','pending') AND cancelled_at IS NULL AND expires_at<=?`, stamp(now), stamp(now))
	if err != nil {
		return fmt.Errorf("expire stale payments for provider command: %w", err)
	}
	return nil
}

func insertPaymentOrderTx(ctx context.Context, tx *sql.Tx, order model.PaymentOrder, now time.Time) error {
	if order.ID == "" || order.Status != "creating" {
		return ErrConflict
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO payment_orders(id,user_id,provider,method_id,provider_rail,status,
		txb_minor,payable_amount,payable_currency,rate_snapshot,rate_direction,provider_payload,expires_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,'{}',?,?,?)`, order.ID, order.UserID, order.Provider, order.MethodID,
		order.ProviderRail, order.Status, order.TXBMinor, order.PayableAmount, order.PayableCurrency, order.RateSnapshot,
		order.RateDirection, stamp(order.ExpiresAt), stamp(now), stamp(now))
	if err != nil {
		return fmt.Errorf("insert durable payment order: %w", err)
	}
	return nil
}
