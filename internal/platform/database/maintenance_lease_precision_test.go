package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMaintenanceLeaseUsesExactFractionalExpiry(t *testing.T) {
	t.Parallel()
	for _, fraction := range []time.Duration{0, 500 * time.Millisecond, 900 * time.Nanosecond} {
		t.Run(fraction.String(), func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
			expiry := base.Add(fraction)
			first, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-11-13", "original", expiry, base.Add(-time.Second), false)
			if err != nil || !acquired {
				t.Fatal("initial lease failed", first, acquired, err)
			}
			before := expiry.Add(-time.Nanosecond)
			if fraction == 500*time.Millisecond {
				before = base
			}
			held, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-11-14", "waiting", expiry.Add(time.Hour), before, true)
			if err != nil || acquired || held != first {
				t.Fatal("live fractional lease overlapped", held, acquired, err)
			}
			takeover := expiry
			if fraction == 0 {
				takeover = expiry.Add(time.Nanosecond)
			}
			next, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-11-14", "replacement", expiry.Add(time.Hour), takeover, true)
			if err != nil || !acquired || next == first {
				t.Fatal("expired lease could not be recovered", next, acquired, err)
			}
			if err := store.CompleteMaintenanceRun(ctx, first, "", nil, nil, takeover); !errors.Is(err, ErrConflict) {
				t.Fatal("expired worker completed replacement's lease", err)
			}
			var running int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM maintenance_runs WHERE status='running'`).Scan(&running); err != nil || running != 1 {
				t.Fatal("takeover left overlapping leases", running, err)
			}
		})
	}
}

func TestDamagedMaintenanceLeaseCannotBeAssumedExpired(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	id, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-11-13", "original", now.Add(time.Hour), now, false)
	if err != nil || !acquired {
		t.Fatal(id, acquired, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE maintenance_runs SET lease_expires_at='damaged' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, acquired, err := store.ClaimMaintenanceRun(ctx, "2026-11-14", "replacement", now.Add(3*time.Hour), now.Add(2*time.Hour), true); err == nil || acquired {
		t.Fatal("damaged lease admitted overlapping work", acquired, err)
	}
	var status, deadline string
	if err := store.DB().QueryRowContext(ctx, `SELECT status,lease_expires_at FROM maintenance_runs WHERE id=$1`, id).Scan(&status, &deadline); err != nil || status != "running" || deadline != "damaged" {
		t.Fatal("failed claim rewrote unknown lease evidence", status, deadline, err)
	}
}

func TestConcurrentFirstMaintenanceClaimsHaveOneOwner(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	peer := NewStore(store.DB())
	var ids [2]string
	var acquired [2]bool
	errs := runTogether(func() error {
		var err error
		ids[0], acquired[0], err = store.ClaimMaintenanceRun(ctx, "2026-11-13", "first", now.Add(time.Hour), now, true)
		return err
	}, func() error {
		var err error
		ids[1], acquired[1], err = peer.ClaimMaintenanceRun(ctx, "2026-11-14", "second", now.Add(time.Hour), now, true)
		return err
	})
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if ids[0] == "" || ids[0] != ids[1] || acquired[0] == acquired[1] {
		t.Fatal("first claims were not serialized", ids, acquired)
	}
}
