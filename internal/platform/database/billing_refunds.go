package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// RefundPayment appends a reversal and unwinds entitlements until the account is solvent or no purchases remain.
func (s *Store) RefundPayment(ctx context.Context, actorID *string, orderID, reason string, now time.Time) (model.PaymentOrder, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PaymentOrder{}, err
	}
	defer func() { _ = tx.Rollback() }()
	order, err := paymentOrderTx(ctx, tx, orderID)
	if err != nil {
		return model.PaymentOrder{}, err
	}
	if order.Status == "refunded" {
		return order, nil
	}
	if order.Status != "paid" {
		return model.PaymentOrder{}, ErrConflict
	}
	refundID, err := ids.New()
	if err != nil {
		return model.PaymentOrder{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO refunds(id,payment_order_id,actor_user_id,txb_minor,reason,status,created_at) VALUES(?,?,?,?,?,'completed',?)`, refundID, order.ID, actorID, order.TXBMinor, reason, stamp(now)); err != nil {
		return model.PaymentOrder{}, fmt.Errorf("record refund: %w", err)
	}
	balance, err := adjustBalanceTx(ctx, tx, order.UserID, -order.TXBMinor, now)
	if err != nil {
		return model.PaymentOrder{}, fmt.Errorf("reverse payment balance: %w", err)
	}
	if _, err := insertLedgerTx(ctx, tx, order.UserID, -order.TXBMinor, balance, "payment_reversal", order.ID, reason, now); err != nil {
		return model.PaymentOrder{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT purchases.id,purchases.charged_txb_minor,purchases.status,combos.name
		FROM purchases JOIN combos ON combos.id=purchases.combo_id WHERE purchases.user_id=?
		AND purchases.status IN ('queued','activating','active') ORDER BY CASE purchases.status WHEN 'queued' THEN 0 ELSE 1 END,purchases.created_at DESC`, order.UserID)
	if err != nil {
		return model.PaymentOrder{}, err
	}
	type cancellation struct {
		id, status, combo string
		price             int64
	}
	cancellations := make([]cancellation, 0)
	for rows.Next() {
		var item cancellation
		if err := rows.Scan(&item.id, &item.price, &item.status, &item.combo); err != nil {
			_ = rows.Close()
			return model.PaymentOrder{}, err
		}
		cancellations = append(cancellations, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return model.PaymentOrder{}, err
	}
	_ = rows.Close()
	cancelledNames := make([]string, 0)
	requiresSync := false
	for _, item := range cancellations {
		if balance >= 0 {
			break
		}
		if _, err := tx.ExecContext(ctx, `UPDATE purchases SET status='cancelled',updated_at=? WHERE id=?`, stamp(now), item.id); err != nil {
			return model.PaymentOrder{}, err
		}
		balance += item.price
		if _, err := tx.ExecContext(ctx, `UPDATE balances SET txb_minor=?,updated_at=? WHERE user_id=?`, balance, stamp(now), order.UserID); err != nil {
			return model.PaymentOrder{}, err
		}
		if _, err := insertLedgerTx(ctx, tx, order.UserID, item.price, balance, "purchase_cancellation", item.id, "cancelled after payment refund", now); err != nil {
			return model.PaymentOrder{}, err
		}
		if item.status != "queued" {
			requiresSync = true
			if err := insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+order.UserID+`"}`, now, now); err != nil {
				return model.PaymentOrder{}, err
			}
		}
		cancelledNames = append(cancelledNames, item.combo)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payment_orders SET status='refunded',provider_payload='{}',payment_url=NULL,qr_payload=NULL,refunded_at=?,updated_at=? WHERE id=?`, stamp(now), stamp(now), order.ID); err != nil {
		return model.PaymentOrder{}, err
	}
	if actorID != nil {
		facts := adminFinanceFacts("payment_refund", -order.TXBMinor, balance, reason, now)
		if len(cancelledNames) > 0 {
			facts[notifications.FactCancelledCombos] = squadSummary(cancelledNames)
		}
		gate := ""
		if requiresSync {
			gate = userSyncGate(order.UserID)
		}
		if _, err := s.insertUserNotificationTx(ctx, tx, "admin:"+refundID+":"+order.UserID, order.UserID,
			jobpayload.UserEventAdminUpdate, gate, facts, now); err != nil {
			return model.PaymentOrder{}, err
		}
	} else {
		facts := map[string]string{
			notifications.FactProvider: order.Provider,
			notifications.FactAmount:   strconv.FormatInt(order.TXBMinor, 10),
			notifications.FactBalance:  strconv.FormatInt(balance, 10),
			notifications.FactTime:     now.UTC().Format(time.RFC3339Nano),
		}
		if len(cancelledNames) > 0 {
			facts[notifications.FactCancelledCombos] = squadSummary(cancelledNames)
		}
		gate := ""
		if requiresSync {
			gate = userSyncGate(order.UserID)
		}
		if _, err := s.insertUserNotificationTx(ctx, tx, "payment-refund:"+refundID, order.UserID,
			jobpayload.UserEventPaymentRefunded, gate, facts, now); err != nil {
			return model.PaymentOrder{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.PaymentOrder{}, err
	}
	return s.PaymentOrderByID(ctx, order.ID)
}

// ListRefunds returns immutable refund records newest first.
func (s *Store) ListRefunds(ctx context.Context, limit int) ([]model.Refund, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,payment_order_id,actor_user_id,txb_minor,reason,status,created_at FROM refunds ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	refunds := make([]model.Refund, 0)
	for rows.Next() {
		var refund model.Refund
		var txbMinor int64
		var created string
		var actor sql.NullString
		if err := rows.Scan(&refund.ID, &refund.PaymentOrderID, &actor, &txbMinor, &refund.Reason, &refund.Status, &created); err != nil {
			return nil, err
		}
		refund.ActorUserID = nullableString(actor)
		refund.TXB = model.TXBMoney(txbMinor)
		refund.CreatedAt, err = parseStamp(created)
		if err != nil {
			return nil, err
		}
		refunds = append(refunds, refund)
	}
	return refunds, rows.Err()
}
