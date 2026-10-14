package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestNotificationGateQueuesOnce(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, 88001)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	tx, err := store.DB().BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	inserted, err := store.insertUserNotificationTx(context.Background(), tx, "expired:purchase-1", user.ID,
		jobpayload.UserEventExpiration, userSyncGate(user.ID), map[string]string{
			notifications.FactCombo: "Pro", notifications.FactExpired: now.Format(time.RFC3339Nano),
		}, now)
	if err != nil || !inserted {
		t.Fatal(err)
	}
	duplicate, err := store.insertUserNotificationTx(context.Background(), tx, "expired:purchase-1", user.ID,
		jobpayload.UserEventExpiration, userSyncGate(user.ID), map[string]string{
			notifications.FactCombo: "changed", notifications.FactExpired: now.Add(time.Hour).Format(time.RFC3339Nano),
		}, now.Add(time.Hour))
	if err != nil || duplicate {
		t.Fatalf("duplicate insert = %t, %v", duplicate, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	assertNotificationCounts(t, store, 1, 0)
	if err := store.ReleaseUserSyncNotifications(context.Background(), user.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := store.ReleaseUserSyncNotifications(context.Background(), user.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertNotificationCounts(t, store, 1, 1)
}

func TestExpiryReminderRequiresNoRenewalOrQueuedSuccessor(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, 88002)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(context.Background(), user.ID, 10_000, "notification-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, store, "Reminder", 100, 3)
	purchase, err := store.CreatePurchase(context.Background(), PurchaseInput{
		UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "notification-reminder",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(context.Background(), purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoRenewal(context.Background(), user.ID, purchase.ID, false, now); err != nil {
		t.Fatal(err)
	}
	if count, err := store.EnqueueExpiryReminderNotifications(context.Background(), purchase.ValidUntil.Add(-49*time.Hour)); err != nil || count != 0 {
		t.Fatalf("early reminder = %d, %v", count, err)
	}
	if count, err := store.EnqueueExpiryReminderNotifications(context.Background(), purchase.ValidUntil.Add(-47*time.Hour)); err != nil || count != 1 {
		t.Fatalf("due reminder = %d, %v", count, err)
	}
	if count, err := store.EnqueueExpiryReminderNotifications(context.Background(), purchase.ValidUntil.Add(-46*time.Hour)); err != nil || count != 0 {
		t.Fatalf("replayed reminder = %d, %v", count, err)
	}
	assertNotificationCounts(t, store, 2, 2)
}

func TestTrafficNotificationRearmsAfterResetOnly(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, 88003)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(context.Background(), user.ID, 10_000, "traffic-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, store, "Traffic", 100, 30)
	purchase, err := store.CreatePurchase(context.Background(), PurchaseInput{
		UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "notification-traffic",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(context.Background(), purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`UPDATE users SET remna_user_id='99' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	firstReset := now
	inserted, err := store.EnqueueTrafficThresholdNotification(context.Background(), "99", 901, 1000, "DAY", &firstReset, now)
	if err != nil || !inserted {
		t.Fatalf("first threshold = %t, %v", inserted, err)
	}
	inserted, err = store.EnqueueTrafficThresholdNotification(context.Background(), "99", 950, 1000, "DAY", &firstReset, now.Add(time.Hour))
	if err != nil || inserted {
		t.Fatalf("same-period threshold = %t, %v", inserted, err)
	}
	secondReset := now.Add(24 * time.Hour)
	inserted, err = store.EnqueueTrafficThresholdNotification(context.Background(), "99", 950, 1000, "DAY", &secondReset, secondReset.Add(time.Hour))
	if err != nil || !inserted {
		t.Fatalf("next-period threshold = %t, %v", inserted, err)
	}
	assertNotificationCounts(t, store, 3, 3)
}

func TestNoResetTrafficNotificationIsLifetimeOnly(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, 88004)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(context.Background(), user.ID, 10_000, "no-reset-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, store, "No reset", 100, 30)
	purchase, err := store.CreatePurchase(context.Background(), PurchaseInput{
		UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "notification-no-reset",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResult(context.Background(), purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`UPDATE users SET remna_user_id='100' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	firstReset, secondReset := now, now.Add(24*time.Hour)
	first, err := store.EnqueueTrafficThresholdNotification(context.Background(), "100", 91, 100, "NO_RESET", &firstReset, now)
	if err != nil || !first {
		t.Fatalf("first NO_RESET threshold = %t, %v", first, err)
	}
	second, err := store.EnqueueTrafficThresholdNotification(context.Background(), "100", 99, 100, "NO_RESET", &secondReset, secondReset)
	if err != nil || second {
		t.Fatalf("replayed NO_RESET threshold = %t, %v", second, err)
	}
	assertNotificationCounts(t, store, 2, 2)
}

func TestImmediateAdminFinanceNotificationIsAtomic(t *testing.T) {
	store := newTestStore(t)
	admin := createTestUser(t, store, 88005)
	user := createTestUser(t, store, 88006)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	entry, err := store.AdjustAdminBalance(context.Background(), admin.ID, user.ID, 125, "admin-adjust-1", "courtesy", now)
	if err != nil || entry.BalanceAfterRaw != 125 {
		t.Fatalf("AdjustAdminBalance() = %+v, %v", entry, err)
	}
	assertNotificationCounts(t, store, 1, 1)
	var amount string
	if err := store.DB().QueryRow(`SELECT json_extract(payload_json,'$.facts.amountMinor') FROM user_notification_events`).Scan(&amount); err != nil {
		t.Fatal(err)
	}
	if amount != "125" {
		t.Fatalf("notification amount = %q", amount)
	}
}

func assertNotificationCounts(t *testing.T, store *Store, events, jobs int) {
	t.Helper()
	var eventCount, jobCount int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM user_notification_events`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind=?`, jobpayload.UserNotificationKind).Scan(&jobCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != events || jobCount != jobs {
		t.Fatalf("notification counts = events %d jobs %d, want %d/%d", eventCount, jobCount, events, jobs)
	}
}
