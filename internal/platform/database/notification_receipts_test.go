package database

import (
	"context"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestImmediatePurchaseQueuesActivationReceiptAfterSync(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 88_301)
	combo := saveTestCombo(t, store, "Immediate", 1250, 30)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 1250, "activation-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID,
		IdempotencyKey: "immediate-notification"}, now)
	if err != nil {
		t.Fatal(err)
	}
	assertNotificationCounts(t, store, 0, 0)
	if err := store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	var kind, charge string
	if err := store.DB().QueryRowContext(ctx, `SELECT kind,json_extract(payload_json,'$.facts.chargeMinor')
		FROM user_notification_events WHERE event_key=?`, "activation:"+purchase.ID).Scan(&kind, &charge); err != nil {
		t.Fatal(err)
	}
	if kind != jobpayload.UserEventPurchaseActivation || charge != "1250" {
		t.Fatalf("activation receipt = %q, charge %q", kind, charge)
	}
	if err := store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	assertNotificationCounts(t, store, 1, 1)
}

func TestPaymentCreditAndProviderRefundQueuePrivateReceiptsOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 88_302)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	order := createTestPaymentOrder(t, store, user.ID, "stars", 2500, now)
	if _, applied, err := store.SettlePayment(ctx, "stars", "receipt-paid", "hash", order.ID, "trade", "", now); err != nil || !applied {
		t.Fatalf("SettlePayment() = %t, %v", applied, err)
	}
	var kind, amount, balance string
	if err := store.DB().QueryRowContext(ctx, `SELECT kind,json_extract(payload_json,'$.facts.amountMinor'),
		json_extract(payload_json,'$.facts.balanceMinor') FROM user_notification_events WHERE event_key=?`,
		"payment:"+order.ID).Scan(&kind, &amount, &balance); err != nil {
		t.Fatal(err)
	}
	if kind != jobpayload.UserEventPaymentCredited || amount != "2500" || balance != "2500" {
		t.Fatalf("payment receipt = %q, amount %q, balance %q", kind, amount, balance)
	}
	if _, err := store.RefundPayment(ctx, nil, order.ID, "Telegram Stars refund", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RefundPayment(ctx, nil, order.ID, "replayed", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT kind,json_extract(payload_json,'$.facts.amountMinor'),
		json_extract(payload_json,'$.facts.balanceMinor') FROM user_notification_events WHERE source_kind='payment-refund'`).
		Scan(&kind, &amount, &balance); err != nil {
		t.Fatal(err)
	}
	if kind != jobpayload.UserEventPaymentRefunded || amount != "2500" || balance != "0" {
		t.Fatalf("refund receipt = %q, amount %q, balance %q", kind, amount, balance)
	}
	assertNotificationCounts(t, store, 2, 2)
}

func TestProviderRefundWaitsForCancelledComboSync(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 88_303)
	combo := saveTestCombo(t, store, "Refunded combo", 2500, 30)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	order := createTestPaymentOrder(t, store, user.ID, "stars", 2500, now)
	if _, applied, err := store.SettlePayment(ctx, "stars", "refund-gate-paid", "hash", order.ID, "trade", "", now); err != nil || !applied {
		t.Fatalf("SettlePayment() = %t, %v", applied, err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID,
		IdempotencyKey: "refund-gate-purchase"}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RefundPayment(ctx, nil, order.ID, "Telegram Stars refund", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var pending bool
	var cancelled string
	if err := store.DB().QueryRowContext(ctx, `SELECT queued_at IS NULL,
		json_extract(payload_json,'$.facts.cancelledCombos') FROM user_notification_events WHERE source_kind='payment-refund'`).
		Scan(&pending, &cancelled); err != nil {
		t.Fatal(err)
	}
	if !pending || cancelled != "1: Refunded combo" {
		t.Fatalf("refund receipt pending = %t, cancelled = %q", pending, cancelled)
	}
	assertNotificationCounts(t, store, 3, 2)
	if err := store.ReleaseUserSyncNotifications(ctx, user.ID, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertNotificationCounts(t, store, 3, 3)
}
