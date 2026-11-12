package database

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestTelegramStarCursorHasOneDurableContinuation(t *testing.T) {
	store, ctx := newTestStore(t), context.Background()
	now := time.Now().UTC()
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			if err := store.EnsureTelegramStarReconciliation(ctx, 123, now); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	cursor, err := store.TelegramStarReconciliationCursor(ctx, 123)
	if err != nil || cursor.NextOffset != 0 || cursor.Generation != 1 || cursor.Anchor != "" {
		t.Fatalf("initial cursor = %+v, %v", cursor, err)
	}
	job, err := store.ClaimOutboxJob(ctx, now)
	if err != nil || job == nil || job.Kind != jobpayload.StarsReconciliationKind {
		t.Fatalf("initial job = %+v, %v", job, err)
	}
	if extra, err := store.ClaimOutboxJob(ctx, now); err != nil || extra != nil {
		t.Fatalf("duplicate initial admission = %+v, %v", extra, err)
	}
	anchor := strings.Repeat("a", 64)
	if err := store.AdvanceTelegramStarReconciliation(ctx, cursor, 100, anchor, now.Add(time.Second), now); err != nil {
		t.Fatal(err)
	}
	if err := store.AdvanceTelegramStarReconciliation(ctx, cursor, 999, strings.Repeat("b", 64), now, now); err != nil {
		t.Fatal("stale completion", err)
	}
	current, err := store.TelegramStarReconciliationCursor(ctx, 123)
	if err != nil || current.Generation != 2 || current.NextOffset != 100 || current.Anchor != anchor {
		t.Fatalf("replayed page changed cursor = %+v, %v", current, err)
	}
	if err := store.CompleteOutboxClaim(ctx, *job, nil, now); err != nil {
		t.Fatal(err)
	}
	job, err = store.ClaimOutboxJob(ctx, now.Add(2*time.Second))
	if err != nil || job == nil {
		t.Fatalf("continuation = %+v, %v", job, err)
	}
	var payload jobpayload.StarsReconciliationPage
	if json.Unmarshal([]byte(job.Payload), &payload) != nil || payload.BotID != 123 || payload.Generation != 2 {
		t.Fatalf("continuation payload = %s", job.Payload)
	}
	next := now.Add(5 * time.Minute)
	if err := store.AdvanceTelegramStarReconciliation(ctx, current, 100, anchor, next, now); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteOutboxClaim(ctx, *job, nil, now); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureTelegramStarReconciliation(ctx, 123, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if early, err := store.ClaimOutboxJob(ctx, now.Add(time.Minute)); err != nil || early != nil {
		t.Fatalf("periodic ensure bypassed polling delay = %+v, %v", early, err)
	}
	// An exhausted page remains resumable at its exact generation and boundary.
	if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='failed' WHERE kind=$1 AND status='pending'`, jobpayload.StarsReconciliationKind); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureTelegramStarReconciliation(ctx, 123, next); err != nil {
		t.Fatal(err)
	}
	job, err = store.ClaimOutboxJob(ctx, next)
	if err != nil || job == nil || json.Unmarshal([]byte(job.Payload), &payload) != nil || payload.Generation != 3 {
		t.Fatalf("exhausted page recovery = %+v, %v", job, err)
	}
	if err := store.EnsureTelegramStarReconciliation(ctx, 456, now); err != nil {
		t.Fatal(err)
	}
	other, err := store.TelegramStarReconciliationCursor(ctx, 456)
	if err != nil || other.NextOffset != 0 || other.Generation != 1 {
		t.Fatalf("separate bot cursor = %+v, %v", other, err)
	}
}

func TestTelegramStarCursorRollsBackWhenContinuationCannotPersist(t *testing.T) {
	store, ctx := newTestStore(t), context.Background()
	now := time.Now().UTC()
	if err := store.EnsureTelegramStarReconciliation(ctx, 123, now); err != nil {
		t.Fatal(err)
	}
	cursor, err := store.TelegramStarReconciliationCursor(ctx, 123)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `ALTER TABLE outbox_jobs ADD CONSTRAINT reject_stars_continuation CHECK(kind<>'telegram_star_reconciliation') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if err := store.AdvanceTelegramStarReconciliation(ctx, cursor, 100, strings.Repeat("a", 64), now, now); err == nil {
		t.Fatal("cursor advanced without durable continuation")
	}
	unchanged, err := store.TelegramStarReconciliationCursor(ctx, 123)
	if err != nil || unchanged != cursor {
		t.Fatalf("cursor changed across rollback = %+v, %v", unchanged, err)
	}
}
