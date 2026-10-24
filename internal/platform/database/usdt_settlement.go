package database

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

// SettleUSDTTransfer consumes verified evidence and uses the established ledger
// transaction. An administrator may override only the original payment window.
func (s *Store) retryAttempt_SettleUSDTTransfer(ctx context.Context, id, orderID, actorID, reason string) (model.PaymentOrder, bool, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	if actorID != "" {
		var role string
		if err := tx.QueryRowContext(ctx, "SELECT role FROM users WHERE id=?", actorID).Scan(&role); err != nil {
			return model.PaymentOrder{}, false, err
		}
		if role != "admin" || len(strings.TrimSpace(reason)) < 1 || len(reason) > 500 {
			return model.PaymentOrder{}, false, usdt.ErrConflict
		}
	}
	t, err := scanUSDTTransfer(tx.QueryRowContext(ctx, usdtTransferSelect+" WHERE id=?", id))
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	if orderID == "" {
		orderID = t.OrderID
	}
	if t.Status == "credited" {
		if t.OrderID != orderID {
			return model.PaymentOrder{}, false, usdt.ErrConflict
		}
		order, err := paymentOrderTx(ctx, tx, orderID)
		if errors.Is(err, ErrNotFound) {
			return model.PaymentOrder{ID: orderID, Provider: usdt.Provider, Status: "paid"}, false, nil
		}
		return order, false, err
	}
	if t.Status == "dismissed" {
		return model.PaymentOrder{}, false, usdt.ErrConflict
	}
	q, err := scanUSDTQuote(tx.QueryRowContext(ctx, usdtQuoteSelect+" WHERE order_id=?", orderID))
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	order, err := paymentOrderTx(ctx, tx, orderID)
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	if order.Provider != usdt.Provider || t.Network != q.Network || t.Token != q.Token || t.Recipient != q.Recipient {
		return model.PaymentOrder{}, false, usdt.ErrConflict
	}
	if t.AtomicAmount != q.AtomicAmount {
		spec, ok := usdt.TokenNetwork(t.Network, t.Token)
		if !ok || !usdt.IsAmountShortfallWithinTolerance(t.AtomicAmount, q.AtomicAmount, spec.Decimals) {
			return model.PaymentOrder{}, false, usdt.ErrConflict
		}
		matched, found, err := matchUSDTQuote(ctx, tx, t)
		if err != nil {
			return model.PaymentOrder{}, false, err
		}
		if !found || matched.OrderID != orderID {
			return model.PaymentOrder{}, false, usdt.ErrConflict
		}
	}
	credited, err := courtesyCreditExistsTx(ctx, tx, order.ID)
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	if order.PaidAt != nil || credited || (order.Status == "failed" && actorID == "") || order.Status == "refunded" {
		if actorID != "" {
			return model.PaymentOrder{}, false, usdt.ErrConflict
		}
		if _, err := tx.ExecContext(ctx, "UPDATE usdt_transfers SET status='review',reason='USDT_ALREADY_CREDITED' WHERE id=?", id); err != nil {
			return model.PaymentOrder{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return model.PaymentOrder{}, false, err
		}
		return order, false, nil
	}
	if actorID == "" {
		if t.Status != "ready" || t.OccurredAt.Before(order.CreatedAt) || !t.OccurredAt.Before(q.ExpiresAt) || (order.CancelledAt != nil && !t.OccurredAt.Before(*order.CancelledAt)) {
			return model.PaymentOrder{}, false, usdt.ErrConflict
		}
	} else {
		if (t.Status != "review" && t.Status != "ready") || t.OccurredAt.Before(order.CreatedAt) {
			return model.PaymentOrder{}, false, usdt.ErrConflict
		}
	}
	now := time.Now().UTC()
	tradeID := "builtin:" + orderID
	chargeID := t.Network + ":" + t.TxHash + ":" + t.EventID
	_, changed, err := s.settlePaymentTx(ctx, tx, usdt.Provider, t.ID, t.ID, orderID, tradeID, chargeID, now)
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	if !changed {
		return model.PaymentOrder{}, false, usdt.ErrConflict
	}
	if _, err := tx.ExecContext(ctx, "UPDATE usdt_transfers SET status='credited',order_id=?,reason='' WHERE id=?", orderID, id); err != nil {
		return model.PaymentOrder{}, false, err
	}
	if actorID != "" {
		auditID, err := ids.New()
		if err != nil {
			return model.PaymentOrder{}, false, err
		}
		detail, err := json.Marshal(map[string]string{"transferId": id, "reason": strings.TrimSpace(reason)})
		if err != nil {
			return model.PaymentOrder{}, false, err
		}
		if err := insertAuditTx(ctx, tx, auditID, &actorID, "usdt.transfer.credit", "payment_order", orderID, string(detail), now); err != nil {
			return model.PaymentOrder{}, false, err
		}
	}
	resolution, err := json.Marshal(map[string]string{"paymentOrderId": orderID, "source": "usdt_transfer"})
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	if err := resolvePaymentCreateOperationTx(ctx, tx, orderID, tradeID, string(resolution), now); err != nil {
		return model.PaymentOrder{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.PaymentOrder{}, false, err
	}
	settled, err := s.PaymentOrderByID(ctx, orderID)
	return settled, true, err
}
