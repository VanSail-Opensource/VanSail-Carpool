package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func TestPurgeDependencyWaitKeepsOneJobAndReviewStopsAutomaticRecovery(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 49861)
	// Admission fixtures use PostgreSQL's microsecond precision. Keep the
	// queue's text timestamp identical when claiming its first due job.
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.HandleTelegramBotStatus(ctx, user.TelegramID, 1, 1, true, false, now); err != nil {
		t.Fatal(err)
	}
	admitTestAccountCleanup(t, store, user, now)
	var original string
	for attempt := range 12 {
		job, err := store.ClaimOutboxJob(ctx, now)
		if err != nil || job == nil || job.Kind != "account_purge" {
			t.Fatal("purge claim", attempt, job, err)
		}
		if attempt == 0 {
			original = job.ID
		}
		if job.ID != original || job.Attempts != 1 {
			t.Fatal("dependency wait replaced/exhausted the durable job", job)
		}
		wait := &outbox.WaitError{Reason: "accounting_pending", After: time.Minute}
		if err = store.CompleteOutboxClaim(ctx, *job, wait, now); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Minute)
		if err = store.EnqueueAccountPurges(ctx, now); err != nil {
			t.Fatal(err)
		}
	}
	job, err := store.ClaimOutboxJob(ctx, now)
	if err != nil || job == nil {
		t.Fatal(job, err)
	}
	if err = store.CompleteOutboxClaim(ctx, *job, &outbox.ReviewError{Reason: "uncertain remote creation"}, now); err != nil {
		t.Fatal(err)
	}
	if err = store.EnqueueAccountPurges(ctx, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	// Refreshing final evidence does not settle an uncertain remote mutation.
	probe := cleanupTestProbe(t, store, user, now.Add(time.Minute), true)
	if result := cleanupTestComplete(t, store, probe, model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}, now.Add(time.Minute)); !result.Admitted {
		t.Fatal(result)
	}
	var status string
	var count int
	if err = store.DB().QueryRowContext(ctx, `SELECT status FROM outbox_jobs WHERE id=$1`, original).Scan(&status); err != nil || status != "failed" {
		t.Fatal(status, err)
	}
	if err = store.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE kind='account_purge'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestPurgeRecoveryReusesExhaustedJobAndFencesPreviousGeneration(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 49862)
	now := time.Now().UTC()
	if err := store.HandleTelegramBotStatus(ctx, user.TelegramID, 1, 1, true, false, now); err != nil {
		t.Fatal(err)
	}
	admitTestAccountCleanup(t, store, user, now)
	var original string
	if err := store.DB().QueryRowContext(ctx, `UPDATE outbox_jobs SET status='failed',attempts=10,last_error='offline participant' WHERE kind='account_purge' RETURNING id`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := store.EnqueueAccountPurges(ctx, now); err != nil {
			t.Fatal(err)
		}
	}
	var count, attempts int
	var id, status string
	if err := store.DB().QueryRowContext(ctx, `SELECT id,status,attempts FROM outbox_jobs WHERE kind='account_purge'`).Scan(&id, &status, &attempts); err != nil || id != original || status != "pending" || attempts != 0 {
		t.Fatal(id, status, attempts, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_generation=2 WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueAccountPurges(ctx, now); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE kind='account_purge'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("unproved generation queued a purge", count, err)
	}
}
