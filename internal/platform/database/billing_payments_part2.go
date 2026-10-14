package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) ListPaymentOrders(ctx context.Context, userID string, limit int) ([]model.PaymentOrder, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := paymentSelect
	args := []any{}
	if userID != "" {
		query += ` WHERE user_id=?`
		args = append(args, userID)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	orders := make([]model.PaymentOrder, 0)
	for rows.Next() {
		order, err := scanPaymentOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

// LatestValidUnpaidPaymentOrder returns the newest checkout that can still be paid.
func (s *Store) LatestValidUnpaidPaymentOrder(ctx context.Context, userID string, now time.Time) (model.PaymentOrder, error) {
	return scanPaymentOrder(s.db.QueryRowContext(ctx, paymentSelect+` WHERE user_id=? AND status='pending'
		AND paid_at IS NULL AND cancelled_at IS NULL AND expires_at>? ORDER BY created_at DESC,id DESC LIMIT 1`,
		userID, stamp(now.UTC())))
}

const paymentSelect = `SELECT id,user_id,provider,method_id,provider_rail,status,txb_minor,payable_amount,payable_currency,rate_snapshot,rate_direction,provider_trade_id,provider_charge_id,payment_url,qr_payload,receiving_address,actual_crypto_amount,actual_crypto_currency,expires_at,paid_at,refunded_at,cancelled_at,cancel_reason,provider_cancel_status,created_at,updated_at FROM payment_orders`

func scanPaymentOrder(row rowScanner) (model.PaymentOrder, error) {
	var order model.PaymentOrder
	var tradeID, chargeID, paymentURL, qr, receivingAddress, actualCryptoAmount, actualCryptoCurrency, paid, refunded, cancelled sql.NullString
	var methodID, providerRail, rateDirection, cancelReason, providerCancelStatus sql.NullString
	var expires, created, updated string
	if err := row.Scan(&order.ID, &order.UserID, &order.Provider, &methodID, &providerRail, &order.Status, &order.TXBMinor, &order.PayableAmount,
		&order.PayableCurrency, &order.RateSnapshot, &rateDirection, &tradeID, &chargeID, &paymentURL, &qr, &receivingAddress, &actualCryptoAmount, &actualCryptoCurrency,
		&expires, &paid, &refunded, &cancelled, &cancelReason, &providerCancelStatus, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.PaymentOrder{}, ErrNotFound
		}
		return model.PaymentOrder{}, fmt.Errorf("scan payment order: %w", err)
	}
	order.TXB = model.TXBMoney(order.TXBMinor)
	order.MethodID = methodID.String
	order.ProviderRail = providerRail.String
	order.RateDirection = rateDirection.String
	order.ProviderTradeID = nullableString(tradeID)
	order.ProviderChargeID = nullableString(chargeID)
	order.PaymentURL = nullableString(paymentURL)
	order.QRPayload = nullableString(qr)
	order.ReceivingAddress = nullableString(receivingAddress)
	order.ActualCryptoAmount = nullableString(actualCryptoAmount)
	order.ActualCryptoCurrency = nullableString(actualCryptoCurrency)
	order.CancelReason = cancelReason.String
	order.ProviderCancelStatus = providerCancelStatus.String
	var err error
	if order.ExpiresAt, err = parseStamp(expires); err != nil {
		return model.PaymentOrder{}, err
	}
	if paid.Valid {
		value, err := parseStamp(paid.String)
		if err != nil {
			return model.PaymentOrder{}, err
		}
		order.PaidAt = &value
	}
	if refunded.Valid {
		value, err := parseStamp(refunded.String)
		if err != nil {
			return model.PaymentOrder{}, err
		}
		order.RefundedAt = &value
	}
	if cancelled.Valid {
		value, err := parseStamp(cancelled.String)
		if err != nil {
			return model.PaymentOrder{}, err
		}
		order.CancelledAt = &value
		if order.Status == "creating" || order.Status == "pending" {
			order.Status = "cancelled"
		}
	}
	if order.CreatedAt, err = parseStamp(created); err != nil {
		return model.PaymentOrder{}, err
	}
	order.UpdatedAt, err = parseStamp(updated)
	return order, err
}
