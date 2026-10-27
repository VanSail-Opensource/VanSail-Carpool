package database

import (
	"context"
	"testing"
	"time"
)

func TestSupersededRenewalDoesNotReportFailure(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 90_099)
	combo := saveTestCombo(t, store, "Renew", 100, 1)
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "superseded-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	old, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "old"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.MarkPurchaseSyncResult(ctx, old.ID, true, now); err != nil {
		t.Fatal(err)
	}
	next, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "next"}, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET auto_renew_enabled=1 WHERE id=?`, old.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.MarkAutoRenewalFailed(ctx, old.ID, AutoRenewalReasonQueuedPurchase, old.ValidUntil); err != nil {
		t.Fatal(err)
	}
	var enabled, notices int
	var reason, nextStatus string
	if err = store.DB().QueryRowContext(ctx, `SELECT auto_renew_enabled,auto_renew_failure_reason FROM purchases WHERE id=?`, old.ID).Scan(&enabled, &reason); err != nil {
		t.Fatal(err)
	}
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE event_key=?`, "auto-renewal-failed:"+old.ID).Scan(&notices); err != nil {
		t.Fatal(err)
	}
	if err = store.DB().QueryRowContext(ctx, `SELECT status FROM purchases WHERE id=?`, next.ID).Scan(&nextStatus); err != nil {
		t.Fatal(err)
	}
	if enabled != 0 || reason != "" || notices != 0 || nextStatus != "queued" {
		t.Fatalf("enabled=%d reason=%q notices=%d next=%q", enabled, reason, notices, nextStatus)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET auto_renew_failure_reason=?,auto_renew_failed_at=? WHERE id=?`, AutoRenewalReasonQueuedPurchase, stamp(old.ValidUntil), old.ID); err != nil {
		t.Fatal(err)
	}
	if failure, err := store.AutoRenewalFailure(ctx, user.ID); err != nil || failure != nil {
		t.Fatalf("superseded failure = %+v, %v", failure, err)
	}
}
