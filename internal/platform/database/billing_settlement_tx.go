package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"strconv"
	"time"
)

// settlePaymentTx shares the existing ledger path with atomic on-chain transfer claims.
func (s *Store) settlePaymentTx(ctx context.Context, tx *sql.Tx, provider, dedupeKey, payloadHash, orderID, tradeID, chargeID string, now time.Time) (model.PaymentOrder, bool, error) {
	var existingOrderID string
	err := tx.QueryRowContext(ctx, `SELECT order_id FROM webhook_events WHERE provider=? AND dedupe_key=?`, provider, dedupeKey).Scan(&existingOrderID)
	if err == nil {
		if existingOrderID != orderID {
			return model.PaymentOrder{}, false, ErrConflict
		}
		order, loadErr := paymentOrderTx(ctx, tx, orderID)
		if loadErr == nil && order.ProviderTradeID != nil && tradeID != "" && *order.ProviderTradeID != tradeID {
			return model.PaymentOrder{}, false, ErrConflict
		}
		return order, false, loadErr
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return model.PaymentOrder{}, false, err
	}
	eventID, err := ids.New()
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO webhook_events(id,provider,dedupe_key,order_id,payload_hash,received_at) VALUES(?,?,?,?,?,?)`, eventID, provider, dedupeKey, orderID, payloadHash, stamp(now)); err != nil {
		return model.PaymentOrder{}, false, err
	}
	order, err := paymentOrderTx(ctx, tx, orderID)
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	if order.Provider != provider {
		return model.PaymentOrder{}, false, ErrConflict
	}
	if order.ProviderTradeID != nil && tradeID != "" && *order.ProviderTradeID != tradeID {
		return model.PaymentOrder{}, false, ErrConflict
	}
	if order.ProviderChargeID != nil && chargeID != "" && *order.ProviderChargeID != chargeID {
		return model.PaymentOrder{}, false, ErrConflict
	}
	courtesyCredited, creditErr := courtesyCreditExistsTx(ctx, tx, order.ID)
	if creditErr != nil {
		return model.PaymentOrder{}, false, creditErr
	}
	if courtesyCredited {
		return model.PaymentOrder{}, false, ErrConflict
	}
	if order.Status == "paid" || order.Status == "refunded" {
		return order, false, nil
	}
	if order.Status != "pending" && order.Status != "creating" && order.Status != "expired" && order.Status != "cancelled" && !(provider == "builtin_usdt" && order.Status == "failed") {
		return model.PaymentOrder{}, false, ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payment_orders SET status='paid',provider_trade_id=COALESCE(NULLIF(?,''),provider_trade_id),provider_charge_id=NULLIF(?,''),
		provider_payload='{}',payment_url=NULL,qr_payload=NULL,paid_at=?,updated_at=? WHERE id=?`, tradeID, chargeID, stamp(now), stamp(now), order.ID); err != nil {
		if isUniqueConstraint(err) {
			return model.PaymentOrder{}, false, ErrConflict
		}
		return model.PaymentOrder{}, false, fmt.Errorf("mark payment paid: %w", err)
	}
	balance, err := adjustBalanceTx(ctx, tx, order.UserID, order.TXBMinor, now)
	if err != nil {
		return model.PaymentOrder{}, false, fmt.Errorf("credit payment: %w", err)
	}
	if _, err := insertLedgerTx(ctx, tx, order.UserID, order.TXBMinor, balance, "payment_credit", order.ID, provider+" payment", now); err != nil {
		return model.PaymentOrder{}, false, err
	}
	if err := settleAffiliateTx(ctx, tx, order.ID, order.UserID, order.TXBMinor, now); err != nil {
		return model.PaymentOrder{}, false, fmt.Errorf("settle affiliate: %w", err)
	}
	announcement, err := paymentSuccessAnnouncementTx(ctx, tx, order)
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	if err := insertOutboxTx(ctx, tx, jobpayload.PaymentSuccessAnnouncementKind, announcement, now, now); err != nil {
		return model.PaymentOrder{}, false, fmt.Errorf("queue payment success announcement: %w", err)
	}
	paidAmount := order.PayableAmount
	if provider == "builtin_usdt" && order.ActualCryptoAmount != nil {
		paidAmount = *order.ActualCryptoAmount
	}
	if _, err := s.insertUserNotificationTx(ctx, tx, "payment:"+order.ID, order.UserID,
		jobpayload.UserEventPaymentCredited, "", map[string]string{
			notifications.FactProvider:      order.Provider,
			notifications.FactPaymentAmount: paidAmount + " " + order.PayableCurrency,
			notifications.FactAmount:        strconv.FormatInt(order.TXBMinor, 10),
			notifications.FactBalance:       strconv.FormatInt(balance, 10),
			notifications.FactTime:          now.UTC().Format(time.RFC3339Nano),
		}, now); err != nil {
		return model.PaymentOrder{}, false, fmt.Errorf("queue payment receipt: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO payment_callback_tombstones(provider,dedupe_key,order_id,
		payload_hash,received_at,processed_at) VALUES(?,?,?,?,?,?)`, provider, dedupeKey, order.ID,
		payloadHash, stamp(now), stamp(now)); err != nil {
		return model.PaymentOrder{}, false, fmt.Errorf("preserve payment callback tombstone: %w", err)
	}
	// The webhook row is only a transient concurrency claim. The compact
	// callback tombstone remains after terminal payment detail is pruned.
	if _, err := tx.ExecContext(ctx, `DELETE FROM webhook_events WHERE id=?`, eventID); err != nil {
		return model.PaymentOrder{}, false, err
	}
	return order, true, nil
}
