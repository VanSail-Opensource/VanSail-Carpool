package database

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestRefundPaymentUnderflowRollsBack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 10010)
	now := time.Date(2026, time.August, 8, 11, 0, 0, 0, time.UTC)
	order := createTestPaymentOrder(t, store, user.ID, "ezpay", 1, now)
	if _, applied, err := store.SettlePayment(ctx, "ezpay", "underflow-paid", "paid", order.ID, "underflow-trade", "", now); err != nil || !applied {
		t.Fatalf("SettlePayment() = (applied %t, err %v)", applied, err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, math.MinInt64, "refund-underflow-a", "test", now.Add(time.Second)); err != nil {
		t.Fatalf("AdjustBalance(MinInt64): %v", err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, -1, "refund-underflow-b", "test", now.Add(2*time.Second)); err != nil {
		t.Fatalf("AdjustBalance(-1): %v", err)
	}
	actorID := user.ID
	if _, err := store.RefundPayment(ctx, &actorID, order.ID, "underflow", now.Add(3*time.Second)); err == nil {
		t.Fatal("RefundPayment(underflow) unexpectedly succeeded")
	}
	current, err := store.PaymentOrderByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("PaymentOrderByID(): %v", err)
	}
	if current.Status != "paid" {
		t.Fatalf("order status = %q, want paid", current.Status)
	}
	assertRowCount(t, store, "refunds", 0)
	balance, err := store.Balance(ctx, user.ID)
	if err != nil {
		t.Fatalf("Balance(): %v", err)
	}
	if balance.Minor != "-9223372036854775808" {
		t.Fatalf("balance = %s, want MinInt64", balance.Minor)
	}
	entries, err := store.ListLedger(ctx, user.ID, 10)
	if err != nil {
		t.Fatalf("ListLedger(): %v", err)
	}
	if got := countLedgerKind(entries, "payment_reversal"); got != 0 {
		t.Fatalf("payment reversal count = %d, want 0", got)
	}
}

func TestRefundCancelsQueuedBeforeActiveAndIsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 10005)
	combo := saveTestCombo(t, store, "refund-plan", 6_000, 30)
	base := time.Date(2026, time.August, 7, 14, 0, 0, 0, time.UTC)
	order := createTestPaymentOrder(t, store, user.ID, "bepusdt", 10_000, base)
	if _, applied, err := store.SettlePayment(ctx, "bepusdt", "paid-event", "paid-hash", order.ID, "trade-refund", "", base); err != nil || !applied {
		t.Fatalf("SettlePayment() = (applied %t, err %v), want (true, nil)", applied, err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 5_000, "preexisting-credit", "test credit", base); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}
	active, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "purchase-cancel-active"}, base.Add(time.Minute))
	if err != nil {
		t.Fatalf("CreatePurchase(active): %v", err)
	}
	queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "purchase-cancel-queued"}, base.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("CreatePurchase(queued): %v", err)
	}
	if active.Status != "activating" || queued.Status != "queued" {
		t.Fatalf("purchase statuses = %q, %q, want activating, queued", active.Status, queued.Status)
	}

	actorID := user.ID
	refunded, err := store.RefundPayment(ctx, &actorID, order.ID, "provider reversal", base.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("RefundPayment(): %v", err)
	}
	if refunded.Status != "refunded" || refunded.RefundedAt == nil {
		t.Fatalf("refunded order = (status %q, refundedAt %v)", refunded.Status, refunded.RefundedAt)
	}

	purchases, err := store.ListPurchases(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListPurchases(): %v", err)
	}
	if len(purchases) != 2 {
		t.Fatalf("purchase count = %d, want 2", len(purchases))
	}
	for _, purchase := range purchases {
		if purchase.Status != "cancelled" {
			t.Fatalf("purchase %s status = %q, want cancelled", purchase.ID, purchase.Status)
		}
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil {
		t.Fatalf("Balance(): %v", err)
	}
	if balance.Minor != "5000" {
		t.Fatalf("balance = %s, want original non-payment credit 5000", balance.Minor)
	}
	entries, err := store.ListLedger(ctx, user.ID, 20)
	if err != nil {
		t.Fatalf("ListLedger(): %v", err)
	}
	wantKinds := map[string]int{
		"admin_adjustment":      1,
		"payment_credit":        1,
		"payment_reversal":      1,
		"purchase_cancellation": 2,
		"purchase_debit":        2,
	}
	for kind, want := range wantKinds {
		if got := countLedgerKind(entries, kind); got != want {
			t.Fatalf("%s ledger count = %d, want %d", kind, got, want)
		}
	}
	var syncJobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_sync_user' AND payload=?`, `{"userId":"`+user.ID+`"}`).Scan(&syncJobs); err != nil {
		t.Fatalf("count refund sync jobs: %v", err)
	}
	if syncJobs != 1 {
		t.Fatalf("refund sync job count = %d, want 1 for the active entitlement only", syncJobs)
	}
	assertRowCount(t, store, "refunds", 1)

	if _, err := store.RefundPayment(ctx, &actorID, order.ID, "duplicate request", base.Add(4*time.Minute)); err != nil {
		t.Fatalf("RefundPayment(replay): %v", err)
	}
	assertRowCount(t, store, "refunds", 1)
	entriesAfterReplay, err := store.ListLedger(ctx, user.ID, 20)
	if err != nil {
		t.Fatalf("ListLedger() after replay: %v", err)
	}
	if len(entriesAfterReplay) != len(entries) {
		t.Fatalf("ledger count after refund replay = %d, want %d", len(entriesAfterReplay), len(entries))
	}
}
