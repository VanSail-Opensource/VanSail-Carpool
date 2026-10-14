package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func TestQueuedPurchaseAndCancellationReceiptsAreOnceOnly(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 90_001)
	combo := saveTestCombo(t, store, "Scheduled", 100, 30)
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "queued-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	first, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "first"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, first.ID, true, now); err != nil {
		t.Fatal(err)
	}
	second, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "second"}, now.Add(time.Hour))
	if err != nil || second.Status != "queued" {
		t.Fatalf("queued purchase = %+v, %v", second, err)
	}
	if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "second"}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	assertEventKind(t, store, "purchase-queued:"+second.ID, jobpayload.UserEventPurchaseQueued)
	if _, err := store.CancelQueuedPurchase(ctx, user.ID, second.ID, "member cancellation", now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	assertEventKind(t, store, "queued-cancellation:"+second.ID, jobpayload.UserEventQueuedCancellation)
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE source_id=?`, second.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("queued purchase event count = %d, %v", count, err)
	}
}

func TestAutomaticRenewalFailureReplacesExpiryNotice(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 90_002)
	combo := saveTestCombo(t, store, "Renew", 100, 1)
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 100, "renew-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "renew-source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkAutoRenewalFailed(ctx, purchase.ID, AutoRenewalReasonInsufficientBalance, purchase.ValidUntil); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkAutoRenewalFailed(ctx, purchase.ID, AutoRenewalReasonInsufficientBalance, purchase.ValidUntil); err != nil {
		t.Fatal(err)
	}
	assertEventKind(t, store, "auto-renewal-failed:"+purchase.ID, jobpayload.UserEventAutoRenewalFailed)
	if err := store.ExpirePurchase(ctx, purchase.ID, purchase.ValidUntil); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE event_key=?`, "expired:"+purchase.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("generic expiry count = %d, %v", count, err)
	}
}

func TestManualResetReportsOnlyTerminalOutcomes(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 90_003)
	combo := saveTestCombo(t, store, "Reset", 301, 30)
	if _, err := store.DB().ExecContext(ctx, `UPDATE combos SET reset_strategy='DAY' WHERE id=?`, combo.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 5000, "reset-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase := createMemberOperationPurchase(t, store, user.ID, combo.ID, "reset-source", now)
	if err := store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	input := providerops.CreateInput{ActorUserID: user.ID, OwnerUserID: user.ID, Kind: purchaseops.OperationResetKind,
		IdempotencyKey: "manual-reset", RequestFingerprint: "1111111111111111",
		Items: []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: purchase.ID}}}
	operation, _, err := store.BeginTrafficReset(ctx, input, purchase.ID, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "purchase", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE source_id=?`, operation.Receipt.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("premature reset event count = %d, %v", count, err)
	}
	if _, err := store.CompleteProviderOperationItem(ctx, operation.Receipt.ID, "purchase", providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: "{}"}, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, operation.Receipt.ID, providerops.Completion{Status: providerops.StatusSucceeded, ResultJSON: "{}"}, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	assertEventKind(t, store, "manual-reset-completed:"+operation.Receipt.ID, jobpayload.UserEventManualResetCompleted)
}

func assertEventKind(t *testing.T, store *Store, key, want string) {
	t.Helper()
	var kind, encoded string
	if err := store.DB().QueryRow(`SELECT kind,payload_json FROM user_notification_events WHERE event_key=?`, key).Scan(&kind, &encoded); err != nil {
		if err == sql.ErrNoRows {
			t.Fatalf("missing notification %s", key)
		}
		t.Fatal(err)
	}
	if kind != want {
		t.Fatalf("notification %s kind = %s, want %s", key, kind, want)
	}
	var payload jobpayload.UserNotification
	if err := json.Unmarshal([]byte(encoded), &payload); err != nil {
		t.Fatal(err)
	}
	if _, err := notifications.Format(payload, time.UTC); err != nil {
		t.Fatalf("persisted notification %s cannot be sent: %v", key, err)
	}
}

func TestGatedAccessNoticeUsesSyncCompletionTime(t *testing.T) {
	created := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	completed := created.Add(time.Hour)
	for _, kind := range []string{jobpayload.UserEventAddonActivated, jobpayload.UserEventMemberRefundCompleted} {
		encoded, err := jobpayload.EncodeUserNotification(jobpayload.UserNotification{
			EventKey: "event", UserID: "user", ChatID: 1, Locale: "en", Kind: kind,
			OccurredAt: stamp(created), Facts: map[string]string{notifications.FactTime: stamp(created)},
		})
		if err != nil {
			t.Fatal(err)
		}
		released, err := notificationPayloadForRelease(encoded, completed)
		if err != nil {
			t.Fatal(err)
		}
		var payload jobpayload.UserNotification
		if err := json.Unmarshal([]byte(released), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Facts[notifications.FactTime] != stamp(completed) {
			t.Fatalf("%s time = %s", kind, payload.Facts[notifications.FactTime])
		}
	}
}
