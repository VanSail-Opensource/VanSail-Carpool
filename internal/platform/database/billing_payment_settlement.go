package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CancelPaymentOrder marks a user-owned unpaid attempt cancelled. It deliberately
// leaves the provider status payable so an authoritative late paid callback can
// still settle and credit the order exactly once.
func (s *Store) CancelPaymentOrder(ctx context.Context, orderID, userID, reason string, now time.Time) (model.PaymentOrder, bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE payment_orders SET cancelled_at=?,cancel_reason=?,provider_payload='{}',payment_url=NULL,
		qr_payload=CASE WHEN receiving_address IS NOT NULL THEN NULL ELSE qr_payload END,updated_at=?
		WHERE id=? AND user_id=? AND status IN ('creating','pending') AND paid_at IS NULL AND cancelled_at IS NULL`,
		stamp(now), reason, stamp(now), orderID, userID)
	if err != nil {
		return model.PaymentOrder{}, false, fmt.Errorf("cancel payment order: %w", err)
	}
	order, loadErr := s.PaymentOrderForUser(ctx, orderID, userID)
	if loadErr != nil {
		return model.PaymentOrder{}, false, loadErr
	}
	affected, _ := result.RowsAffected()
	if affected == 0 && order.Status != "cancelled" && order.Status != "paid" && order.Status != "refunded" {
		return model.PaymentOrder{}, false, ErrConflict
	}
	return order, affected == 1, nil
}

// SetPaymentProviderCancellation records the redacted result of a best-effort
// provider cancellation without changing the authoritative settlement state.
func (s *Store) SetPaymentProviderCancellation(ctx context.Context, orderID, status string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE payment_orders SET provider_cancel_status=?,updated_at=? WHERE id=?`, status, stamp(now), orderID)
	if err != nil {
		return fmt.Errorf("record provider cancellation: %w", err)
	}
	return nil
}

// SettlePayment records one authoritative provider event and credits the exact requested TXB amount once.
func (s *Store) SettlePayment(ctx context.Context, provider, dedupeKey, payloadHash, orderID, tradeID, chargeID string, now time.Time) (model.PaymentOrder, bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	order, changed, err := s.settlePaymentTx(ctx, tx, provider, dedupeKey, payloadHash, orderID, tradeID, chargeID, now)
	if err != nil || !changed {
		return order, changed, err
	}
	if err := tx.Commit(); err != nil {
		return model.PaymentOrder{}, false, err
	}
	settled, err := s.PaymentOrderByID(ctx, orderID)
	return settled, true, err
}

func paymentOrderTx(ctx context.Context, tx *sql.Tx, id string) (model.PaymentOrder, error) {
	return scanPaymentOrder(tx.QueryRowContext(ctx, paymentSelect+` WHERE id=?`, id))
}
