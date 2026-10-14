package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestAuditRetentionKeepsAllEvents(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	for index := 0; index < 205; index++ {
		if err := store.AppendAudit(ctx, nil, "test", "record", fmt.Sprintf("%03d", index), `{}`, base.Add(time.Duration(index)*time.Second)); err != nil {
			t.Fatalf("AppendAudit(%d): %v", index, err)
		}
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events`).Scan(&count); err != nil {
		t.Fatalf("count audit events: %v", err)
	}
	if count != 205 {
		t.Fatalf("audit count = %d, want 205", count)
	}
	var oldestTarget string
	if err := store.DB().QueryRowContext(ctx, `SELECT target_id FROM audit_events ORDER BY created_at LIMIT 1`).Scan(&oldestTarget); err != nil {
		t.Fatalf("oldest audit event: %v", err)
	}
	if oldestTarget != "000" {
		t.Fatalf("oldest retained target = %q, want 000", oldestTarget)
	}
}

func TestPaymentCreationDoesNotPruneTerminalOrLiveOrders(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, store, 29001)
	base := time.Now().UTC()
	created := make([]model.PaymentOrder, 0, 200)
	for index := 0; index < 200; index++ {
		order, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{
			UserID: user.ID, Provider: "stars", Status: "pending", TXBMinor: 100,
			PayableAmount: "1", PayableCurrency: "XTR", RateSnapshot: "1",
			ExpiresAt: base.Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreatePaymentOrder(%d): %v", index, err)
		}
		created = append(created, order)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET status='failed' WHERE id=?`, created[0].ID); err != nil {
		t.Fatalf("mark terminal order: %v", err)
	}
	newOrder, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{
		UserID: user.ID, Provider: "stars", Status: "pending", TXBMinor: 100,
		PayableAmount: "1", PayableCurrency: "XTR", RateSnapshot: "1",
		ExpiresAt: base.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreatePaymentOrder(after terminal): %v", err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM payment_orders`).Scan(&count); err != nil {
		t.Fatalf("count payment orders: %v", err)
	}
	if count != 201 {
		t.Fatalf("payment count = %d, want 201", count)
	}
	if _, err := store.PaymentOrderByID(ctx, created[0].ID); err != nil {
		t.Fatalf("terminal order was pruned during creation: %v", err)
	}
	if _, err := store.PaymentOrderByID(ctx, newOrder.ID); err != nil {
		t.Fatalf("new order lookup: %v", err)
	}
}

func TestCancelledPaymentCanSettleLateExactlyOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 27201)
	now := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	order, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{
		UserID: user.ID, Provider: "bepusdt", MethodID: "bepusdt:usdt.trc20", ProviderRail: "usdt.trc20",
		Status: "pending", TXBMinor: 500, PayableAmount: "1.00", PayableCurrency: "USD", RateSnapshot: "5",
		RateDirection: "txb_per_currency", ExpiresAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreatePaymentOrder(): %v", err)
	}
	cancelled, changed, err := store.CancelPaymentOrder(ctx, order.ID, user.ID, "user", now)
	if err != nil || !changed || cancelled.Status != "cancelled" {
		t.Fatalf("CancelPaymentOrder() = (%+v, %t, %v)", cancelled, changed, err)
	}
	if err := store.ExpirePaymentOrder(ctx, order.ID, "bepusdt", now.Add(30*time.Second)); err != nil {
		t.Fatalf("ExpirePaymentOrder(cancelled replay): %v", err)
	}
	if stillCancelled, err := store.PaymentOrderByID(ctx, order.ID); err != nil || stillCancelled.Status != "cancelled" {
		t.Fatalf("cancelled order after timeout replay = (%+v, %v)", stillCancelled, err)
	}
	settled, applied, err := store.SettlePayment(ctx, "bepusdt", "block-1", "hash", order.ID, "trade-1", "block-1", now.Add(time.Minute))
	if err != nil || !applied || settled.Status != "paid" {
		t.Fatalf("SettlePayment(late) = (%+v, %t, %v)", settled, applied, err)
	}
	_, applied, err = store.SettlePayment(ctx, "bepusdt", "block-1", "hash", order.ID, "trade-1", "block-1", now.Add(2*time.Minute))
	if err != nil || applied {
		t.Fatalf("SettlePayment(replay) = applied %t, err %v", applied, err)
	}
	var balance int64
	if err := store.DB().QueryRowContext(ctx, `SELECT txb_minor FROM balances WHERE user_id=?`, user.ID).Scan(&balance); err != nil {
		t.Fatalf("read balance: %v", err)
	}
	if balance != 500 {
		t.Fatalf("balance = %d, want 500", balance)
	}
}

func TestPaymentCreationNeverPrunesCancelledOrders(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 27202)
	now := time.Now().UTC()
	order, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{
		UserID: user.ID, Provider: "stars", Status: "pending", TXBMinor: 100,
		PayableAmount: "1", PayableCurrency: "XTR", RateSnapshot: "1",
		ExpiresAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreatePaymentOrder(): %v", err)
	}
	if _, _, err := store.CancelPaymentOrder(ctx, order.ID, user.ID, "user", now); err != nil {
		t.Fatalf("CancelPaymentOrder(): %v", err)
	}
	if _, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{
		UserID: user.ID, Provider: "stars", Status: "pending", TXBMinor: 100,
		PayableAmount: "1", PayableCurrency: "XTR", RateSnapshot: "1",
		ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreatePaymentOrder(after cancellation): %v", err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET expires_at=? WHERE id=?`, stamp(now.Add(-time.Second)), order.ID); err != nil {
		t.Fatalf("expire cancelled order: %v", err)
	}
	if _, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{
		UserID: user.ID, Provider: "stars", Status: "pending", TXBMinor: 100,
		PayableAmount: "1", PayableCurrency: "XTR", RateSnapshot: "1",
		ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreatePaymentOrder(after provider expiry): %v", err)
	}
	if _, err := store.PaymentOrderByID(ctx, order.ID); err != nil {
		t.Fatalf("cancelled order was pruned during creation: %v", err)
	}
}
