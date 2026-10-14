package database

import (
	"context"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestSetAutomaticRenewalAllowsSettledExpiredTerm(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_004)
	combo := saveTestCombo(t, store, "automatic-expired", 100, 1)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 100, "expired-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "automatic-expired"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, purchase.ID, false, now); err != nil {
		t.Fatalf("SetAutoRenewal(disable): %v", err)
	}
	if err := store.EnqueueDueEntitlementTransitions(ctx, purchase.ValidUntil); err != nil {
		t.Fatalf("EnqueueDueEntitlementTransitions(): %v", err)
	}
	if err := store.MarkRolloverProcessing(ctx, purchase.ID, purchase.ValidUntil); err != nil {
		t.Fatalf("MarkRolloverProcessing(): %v", err)
	}
	if _, err := store.FinalizeRollover(ctx, purchase.ID, 100, 100, "", purchase.ValidUntil); err != nil {
		t.Fatalf("FinalizeRollover(): %v", err)
	}
	var kind string
	if err := store.DB().QueryRowContext(ctx, `SELECT kind FROM user_notification_events WHERE event_key=?`,
		"expired:"+purchase.ID).Scan(&kind); err != nil || kind != jobpayload.UserEventExpiration {
		t.Fatalf("expiration notification = %q, %v", kind, err)
	}
	assertNotificationCounts(t, store, 1, 0)
	if err := store.ReleaseUserSyncNotifications(ctx, user.ID, purchase.ValidUntil); err != nil {
		t.Fatal(err)
	}
	assertNotificationCounts(t, store, 1, 1)
	plan, err := store.AutoRenewalPlan(ctx, user.ID, purchase.ID, purchase.ValidUntil)
	if err != nil || plan.IneligibleReason != "" {
		t.Fatalf("AutoRenewalPlan() = (%+v, %v), want eligible settled term", plan, err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, purchase.ID, true, purchase.ValidUntil); err != nil {
		t.Fatalf("SetAutoRenewal(enable settled expired): %v", err)
	}
}
