package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestRecoverOutboxReleasesAbandonedLease(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	if err := store.EnqueueOutbox(ctx, "xnode_sync_user", `{"userId":"user-1"}`, now); err != nil {
		t.Fatalf("EnqueueOutbox(): %v", err)
	}
	claimed, err := store.ClaimOutboxJob(ctx, now)
	if err != nil || claimed == nil || claimed.Status != "processing" {
		t.Fatalf("ClaimOutboxJob(first) = %+v, %v", claimed, err)
	}
	if err := store.RecoverOutbox(ctx, now.Add(time.Second), now.Add(time.Second)); err != nil {
		t.Fatalf("RecoverOutbox(): %v", err)
	}
	reclaimed, err := store.ClaimOutboxJob(ctx, now.Add(time.Second))
	if err != nil || reclaimed == nil || reclaimed.ID != claimed.ID || reclaimed.Attempts != 2 {
		t.Fatalf("ClaimOutboxJob(recovered) = %+v, %v", reclaimed, err)
	}
}

func TestCompleteOutboxJobRemovesSuccessAndRetainsFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name       string
		jobErr     error
		wantStatus string
	}{
		{name: "success", wantStatus: "missing"},
		{name: "failure", jobErr: errors.New("provider unavailable"), wantStatus: "pending"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := store.EnqueueOutbox(ctx, "xnode_sync_user", `{"userId":"user-1"}`, now); err != nil {
				t.Fatalf("EnqueueOutbox(): %v", err)
			}
			claimed, err := store.ClaimOutboxJob(ctx, now)
			if err != nil || claimed == nil {
				t.Fatalf("ClaimOutboxJob() = %+v, %v", claimed, err)
			}
			if err := store.CompleteOutboxJob(ctx, claimed.ID, claimed.Attempts, test.jobErr, now.Add(time.Second)); err != nil {
				t.Fatalf("CompleteOutboxJob(): %v", err)
			}

			var status string
			err = store.DB().QueryRowContext(ctx, `SELECT status FROM outbox_jobs WHERE id=?`, claimed.ID).Scan(&status)
			if test.wantStatus == "missing" {
				if !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("completed job lookup error = %v, want sql.ErrNoRows", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("failed job lookup: %v", err)
			}
			if status != test.wantStatus {
				t.Fatalf("failed job status = %q, want %q", status, test.wantStatus)
			}
		})
	}
}

func TestMarkupMigrationRetriesOnlyDefinitiveTelegramRejections(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := stamp(time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC))
	for _, item := range []struct{ id, payload, failure string }{
		{id: "parse", payload: `{"eventKey":"parse"}`, failure: "telegram sendMessage failed (http=400 api=400): Bad Request: can't parse entities"},
		{id: "blocked", payload: `{"eventKey":"blocked"}`, failure: "telegram sendMessage failed (http=403 api=403): bot was blocked by the user"},
	} {
		if _, err := store.DB().ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,payload,status,attempts,available_at,last_error,created_at,updated_at)
			VALUES(?,'telegram_user_notification',?,'failed',10,?,?,?,?)`, item.id, item.payload, now, item.failure, now, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DB().ExecContext(ctx, `DELETE FROM schema_migrations WHERE version='049_retry_rejected_telegram_markup.sql'`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, store.DB()); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, wanted string }{{"parse", "pending"}, {"blocked", "failed"}} {
		var status string
		if err := store.DB().QueryRowContext(ctx, `SELECT status FROM outbox_jobs WHERE id=?`, item.id).Scan(&status); err != nil || status != item.wanted {
			t.Fatalf("job %s status = %q, err %v, want %q", item.id, status, err, item.wanted)
		}
	}
}
