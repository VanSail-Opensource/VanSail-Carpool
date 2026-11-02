package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMaintenanceRunLeaseUsesRunIDAsFencingToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)
	firstID, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-08-18", "worker-a", now.Add(5*time.Minute), now, false)
	if err != nil || !acquired || firstID == "" {
		t.Fatalf("ClaimMaintenanceRun(first) = (%q, %t, %v)", firstID, acquired, err)
	}
	heldID, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-08-18", "worker-b", now.Add(6*time.Minute), now.Add(time.Minute), false)
	if err != nil || acquired || heldID != firstID {
		t.Fatalf("ClaimMaintenanceRun(held) = (%q, %t, %v), want %q false", heldID, acquired, err, firstID)
	}

	takeoverAt := now.Add(6 * time.Minute)
	secondID, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-08-18", "worker-b", takeoverAt.Add(5*time.Minute), takeoverAt, false)
	if err != nil || !acquired || secondID == "" || secondID == firstID {
		t.Fatalf("ClaimMaintenanceRun(takeover) = (%q, %t, %v), first %q", secondID, acquired, err, firstID)
	}
	if err := store.CompleteMaintenanceRun(ctx, firstID, "", map[string]int64{"sessions": 1}, nil, takeoverAt); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale CompleteMaintenanceRun() error = %v, want ErrConflict", err)
	}
	if err := store.CompleteMaintenanceRun(ctx, secondID, "", map[string]int64{"sessions": 1}, nil, takeoverAt); err != nil {
		t.Fatalf("CompleteMaintenanceRun(active): %v", err)
	}
	completedID, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-08-18", "worker-c", takeoverAt.Add(time.Hour), takeoverAt.Add(time.Minute), false)
	if err != nil || acquired || completedID != secondID {
		t.Fatalf("ClaimMaintenanceRun(completed) = (%q, %t, %v), want %q false", completedID, acquired, err, secondID)
	}
}

func TestMaintenanceRunAllowsForcedSameDayRerun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)
	firstID, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-08-18", "worker-a", now.Add(time.Hour), now, false)
	if err != nil || !acquired {
		t.Fatalf("first claim = (%q, %t, %v)", firstID, acquired, err)
	}
	if err := store.CompleteMaintenanceRun(ctx, firstID, "", nil, nil, now); err != nil {
		t.Fatalf("complete first run: %v", err)
	}
	secondID, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-08-18", "worker-b", now.Add(2*time.Hour), now.Add(time.Minute), true)
	if err != nil || !acquired || secondID == firstID {
		t.Fatalf("forced claim = (%q, %t, %v), first %q", secondID, acquired, err, firstID)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM maintenance_runs WHERE local_date=$1`, "2026-08-18").Scan(&count); err != nil {
		t.Fatalf("count maintenance runs: %v", err)
	}
	if count != 2 {
		t.Fatalf("maintenance run count=%d, want 2", count)
	}
}

func TestMaintenanceRunSerializesActiveRunsAcrossLocalDates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 23, 59, 0, 0, time.UTC)
	firstID, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-08-18", "worker-a", now.Add(time.Hour), now, false)
	if err != nil || !acquired {
		t.Fatalf("first claim = (%q, %t, %v)", firstID, acquired, err)
	}
	secondID, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-08-19", "worker-b", now.Add(2*time.Hour), now.Add(time.Minute), true)
	if err != nil || acquired || secondID != firstID {
		t.Fatalf("next-date claim = (%q, %t, %v), want held %q", secondID, acquired, err, firstID)
	}
}
