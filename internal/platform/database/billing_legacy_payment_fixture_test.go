package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// CreatePaymentOrder seeds historical checkout states for settlement, retention
// and migration fixtures. Production admission uses BeginPaymentOrderOperation,
// which binds account ownership and an outbox receipt in the same transaction.
func (s *Store) CreatePaymentOrder(ctx context.Context, order model.PaymentOrder) (model.PaymentOrder, error) {
	for retryIndex := 0; ; retryIndex++ {
		r0, r1 := s.retryAttempt_CreatePaymentOrder(ctx, order)
		if !s.retryTransaction(ctx, r1, retryIndex) {
			return r0, r1
		}
	}
}

func (s *Store) retryAttempt_CreatePaymentOrder(ctx context.Context, order model.PaymentOrder) (model.PaymentOrder, error) {

	if order.ID == "" {
		var err error
		order.ID, err = ids.New()
		if err != nil {
			return model.PaymentOrder{}, err
		}
	}
	now := time.Now().UTC()
	if order.Status == "" {
		order.Status = "creating"
	}
	if order.RateDirection == "" {
		order.RateDirection = "currency_per_txb"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PaymentOrder{}, fmt.Errorf("begin payment order retention: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE payment_orders SET status='expired',provider_payload='{}',payment_url=NULL,
		qr_payload=CASE WHEN receiving_address IS NOT NULL THEN NULL ELSE qr_payload END,updated_at=$1
		WHERE provider<>'bepusdt' AND status IN ('creating','pending') AND cancelled_at IS NULL AND expires_at<=$2`, stamp(now), stamp(now)); err != nil {
		return model.PaymentOrder{}, fmt.Errorf("expire stale payment orders before insert: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO payment_orders(id,user_id,provider,method_id,provider_rail,status,txb_minor,payable_amount,payable_currency,rate_snapshot,rate_direction,provider_payload,expires_at,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'{}',$12,$13,$14)`, order.ID, order.UserID, order.Provider, order.MethodID, order.ProviderRail, order.Status, order.TXBMinor, order.PayableAmount,
		order.PayableCurrency, order.RateSnapshot, order.RateDirection, stamp(order.ExpiresAt), stamp(now), stamp(now))
	if err != nil {
		return model.PaymentOrder{}, fmt.Errorf("create payment order: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.PaymentOrder{}, fmt.Errorf("commit payment order: %w", err)
	}
	return s.PaymentOrderByID(ctx, order.ID)
}
