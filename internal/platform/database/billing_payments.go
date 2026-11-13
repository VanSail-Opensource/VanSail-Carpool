package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

// UpdatePaymentCheckout stores the provider response without changing the requested TXB amount.

func (s *Store) UpdatePaymentCheckoutDetails(ctx context.Context, orderID string, tradeID, paymentURL, qrPayload, receivingAddress, actualCryptoAmount, actualCryptoCurrency *string, payableAmount, payableCurrency string, expiresAt time.Time) (model.PaymentOrder, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE payment_orders SET status='pending',provider_trade_id=$1,payment_url=$2,qr_payload=$3,receiving_address=$4,actual_crypto_amount=$5,actual_crypto_currency=$6,payable_amount=$7,payable_currency=$8,provider_payload='{}',expires_at=$9,updated_at=$10 WHERE id=$11 AND status='creating'`,
		tradeID, paymentURL, qrPayload, receivingAddress, actualCryptoAmount, actualCryptoCurrency, payableAmount, payableCurrency, stamp(expiresAt), stamp(time.Now().UTC()), orderID)
	if err != nil {
		return model.PaymentOrder{}, fmt.Errorf("update payment checkout: %w", err)
	}
	return s.PaymentOrderByID(ctx, orderID)
}

// UpdatePaymentCheckout preserves the original repository contract for
// adapters that do not return a separate receiving address.

func (s *Store) UpdatePaymentCheckout(ctx context.Context, orderID string, tradeID, paymentURL, qrPayload *string, payableAmount, payableCurrency string, expiresAt time.Time) (model.PaymentOrder, error) {
	return s.UpdatePaymentCheckoutDetails(ctx, orderID, tradeID, paymentURL, qrPayload, nil, nil, nil, payableAmount, payableCurrency, expiresAt)
}

// FailPaymentOrder records a provider creation failure without retaining provider data.

func (s *Store) FailPaymentOrder(ctx context.Context, orderID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE payment_orders SET status='failed',provider_payload='{}',payment_url=NULL,qr_payload=NULL,updated_at=$1 WHERE id=$2 AND status='creating'`, stamp(time.Now().UTC()), orderID)
	return err
}

// ExpirePaymentOrder records an authoritative provider timeout without affecting balance.

func (s *Store) ExpirePaymentOrder(ctx context.Context, orderID, provider string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE payment_orders SET status='expired',provider_payload='{}',payment_url=NULL,
		qr_payload=CASE WHEN receiving_address IS NOT NULL THEN NULL ELSE qr_payload END,updated_at=$1
		WHERE id=$2 AND provider=$3 AND status IN ('creating','pending') AND cancelled_at IS NULL`, stamp(now), orderID, provider)
	if err != nil {
		return fmt.Errorf("expire payment order: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		order, loadErr := s.PaymentOrderByID(ctx, orderID)
		if loadErr != nil {
			return loadErr
		}
		if order.Status != "paid" && order.Status != "refunded" && order.Status != "expired" && order.Status != "cancelled" {
			return ErrConflict
		}
	}
	return nil
}

// ExpireStalePaymentOrders closes locally expired attempts without crediting them.

func (s *Store) ExpireStalePaymentOrders(ctx context.Context, now time.Time) error {
	_, err := hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, expireStalePaymentsTx(ctx, tx, now)
	})
	return err
}

// PaymentOrderByID loads one payment order.

func (s *Store) PaymentOrderByID(ctx context.Context, id string) (model.PaymentOrder, error) {
	return scanPaymentOrder(s.db.QueryRowContext(ctx, paymentSelect+` WHERE id=$1`, id))
}

// PaymentOrderForUser prevents order-ID enumeration across accounts.

func (s *Store) PaymentOrderForUser(ctx context.Context, id, userID string) (model.PaymentOrder, error) {
	return scanPaymentOrder(s.db.QueryRowContext(ctx, paymentSelect+` WHERE id=$1 AND user_id=$2`, id, userID))
}

// ListPaymentOrders returns recent payment attempts.
