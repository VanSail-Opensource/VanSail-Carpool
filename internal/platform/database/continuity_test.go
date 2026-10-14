package database

import (
	"context"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestQueuedPurchaseSchedulesThreeMinuteContinuity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 49_001)
	combo := saveTestCombo(t, store, "continuity", 100, 30)
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "continuity-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	current, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "continuity-current"}, now)
	if err != nil {
		t.Fatal(err)
	}
	queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "continuity-next"}, now.Add(time.Minute))
	if err != nil || queued.Status != "queued" {
		t.Fatalf("CreatePurchase(queued) = (%+v, %v)", queued, err)
	}
	var availableValue string
	payload := `{"purchaseId":"` + queued.ID + `"}`
	if err := store.DB().QueryRowContext(ctx, `SELECT available_at FROM outbox_jobs WHERE kind=? AND payload=?`, jobpayload.ContinuityKind, payload).Scan(&availableValue); err != nil {
		t.Fatalf("load continuity job: %v", err)
	}
	available, err := parseStamp(availableValue)
	if err != nil || !available.Equal(queued.ValidFrom.Add(-EntitlementContinuityLead)) {
		t.Fatalf("continuity available_at = %s, %v", available, err)
	}
	projection, err := store.ContinuityEntitlement(ctx, queued.ID, queued.ValidFrom.Add(-time.Minute))
	if err != nil || projection == nil || projection.ID != current.ID || !projection.ValidUntil.Equal(queued.ValidUntil) {
		t.Fatalf("ContinuityEntitlement() = (%+v, %v)", projection, err)
	}
	projection, err = store.ContinuityEntitlement(ctx, queued.ID, queued.ValidFrom)
	if err != nil || projection == nil || projection.ID != current.ID || !projection.ValidUntil.Equal(queued.ValidUntil) {
		t.Fatalf("ContinuityEntitlement(boundary) = (%+v, %v)", projection, err)
	}
}

func TestContinuityBacklogRepairsFarFutureQueuedTermsOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 49_002)
	combo := saveTestCombo(t, store, "continuity-backfill", 100, 30)
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "continuity-backfill", "seed", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "backfill-current"}, now); err != nil {
		t.Fatal(err)
	}
	queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "backfill-next"}, now)
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"purchaseId":"` + queued.ID + `"}`
	if _, err := store.DB().ExecContext(ctx, `DELETE FROM outbox_jobs WHERE kind=? AND payload=?`, jobpayload.ContinuityKind, payload); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueContinuityBacklog(ctx, now); err != nil {
		t.Fatalf("EnqueueContinuityBacklog(): %v", err)
	}
	if err := store.EnqueueContinuityBacklog(ctx, now.Add(time.Minute)); err != nil {
		t.Fatalf("EnqueueContinuityBacklog(replay): %v", err)
	}
	var count int
	var availableRaw string
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*),MIN(available_at) FROM outbox_jobs WHERE kind=? AND payload=?`,
		jobpayload.ContinuityKind, payload).Scan(&count, &availableRaw); err != nil {
		t.Fatal(err)
	}
	available, err := parseStamp(availableRaw)
	if err != nil || count != 1 || !available.Equal(queued.ValidFrom.Add(-EntitlementContinuityLead)) {
		t.Fatalf("backfilled continuity = count %d at %s, %v", count, available, err)
	}
}
