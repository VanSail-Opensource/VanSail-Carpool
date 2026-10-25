package database

import (
	"context"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestQueuedPurchaseUsesNativeContinuityWithoutPreparationJob(t *testing.T) {
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
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=?`, jobpayload.ContinuityKind).Scan(&count); err != nil || count != 0 {
		t.Fatal("retired continuity job was produced", count, err)
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

func TestRetireContinuityJobsPreservesClaimsAndHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	at := time.Now().UTC()
	for _, status := range []string{"pending", "failed", "processing", "done"} {
		if err := store.EnqueueOutbox(ctx, jobpayload.ContinuityKind, `{"purchaseId":"`+status+`"}`, at.Add(24*time.Hour)); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status=? WHERE kind=? AND json_extract(payload,'$.purchaseId')=?`, status, jobpayload.ContinuityKind, status); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := store.RetireContinuityJobs(ctx, at); err != nil {
			t.Fatal(err)
		}
	}
	var done, processing int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND status='done'`, jobpayload.ContinuityKind).Scan(&done); err != nil || done != 3 {
		t.Fatal(done, err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND status='processing'`, jobpayload.ContinuityKind).Scan(&processing); err != nil || processing != 1 {
		t.Fatal(processing, err)
	}
}
