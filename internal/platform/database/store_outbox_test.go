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
			if err := store.CompleteOutboxClaim(ctx, *claimed, test.jobErr, now.Add(time.Second)); err != nil {
				t.Fatalf("CompleteOutboxClaim(): %v", err)
			}

			var status string
			err = store.DB().QueryRowContext(ctx, `SELECT status FROM outbox_jobs WHERE id=$1`, claimed.ID).Scan(&status)
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
