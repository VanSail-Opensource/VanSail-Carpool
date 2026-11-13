package database

import (
	"context"
	"testing"
	"time"
)

func TestLedgerListsAndPagesUseExactChronology(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 971010)
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	first, err := store.AdjustBalance(ctx, user.ID, 100, "whole-second", "earlier", at)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.AdjustBalance(ctx, user.ID, 200, "fractional-second", "later", at.Add(time.Nanosecond))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := store.ListLedger(ctx, user.ID, 2)
	if err != nil || len(entries) != 2 || entries[0].ID != second.ID || entries[1].ID != first.ID {
		t.Fatal("ledger chronology", entries, err)
	}
	page, cursor, err := store.ListLedgerPage(ctx, user.ID, "", 1)
	if err != nil || len(page) != 1 || page[0].ID != second.ID || cursor == nil {
		t.Fatal("first ledger page", page, cursor, err)
	}
	page, cursor, err = store.ListLedgerPage(ctx, user.ID, *cursor, 1)
	if err != nil || len(page) != 1 || page[0].ID != first.ID || cursor != nil {
		t.Fatal("second ledger page", page, cursor, err)
	}
}

func TestPaymentRefundCancelsNewestQueuedPurchaseAtFractionalBoundary(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 971011)
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "ordered reversal", 100, 30)
	order := createTestPaymentOrder(t, store, user.ID, "ezpay", 100, at.Add(-time.Hour))
	if _, applied, err := store.SettlePayment(ctx, "ezpay", "chronology-callback", "proof", order.ID, "chronology-trade", "chronology-charge", at.Add(-time.Hour)); err != nil || !applied {
		t.Fatal("payment fixture", applied, err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 200, "chronology-seed", "seed", at.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	current, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "current"}, at.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, current.ID, true, at.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	earlier, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "earlier"}, at)
	if err != nil || earlier.Status != "queued" {
		t.Fatal("earlier queued fixture", earlier.Status, err)
	}
	later, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "later"}, at.Add(time.Nanosecond))
	if err != nil || later.Status != "queued" {
		t.Fatal("later queued fixture", later.Status, err)
	}
	if _, err := store.RefundPayment(ctx, nil, order.ID, "confirmed provider refund", at.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	retained, err := store.PurchaseByID(ctx, earlier.ID)
	if err != nil || retained.Status != "queued" {
		t.Fatal("refund cancelled an earlier paid term", retained.Status, err)
	}
	reversed, err := store.PurchaseByID(ctx, later.ID)
	if err != nil || reversed.Status != "cancelled" {
		t.Fatal("refund did not unwind the newest term", reversed.Status, err)
	}
	assertBalance(t, store, user.ID, "0")
}
