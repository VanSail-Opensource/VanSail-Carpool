package database

import (
	"context"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestDelayedActivationExpiryPreservesAnExtendedTerm(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	boundary := time.Date(2026, 11, 13, 16, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "Delayed expiry", 100, 30)
	user, purchase := createAdminWorkflowPurchase(t, store, 89111, combo, boundary.Add(-time.Hour))
	// A delayed worker still holds its earlier expiry, while a committed edit
	// has moved the authoritative term to a later fractional instant.
	extended := boundary.Add(time.Hour + 900*time.Nanosecond)
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, stamp(extended), purchase.ID); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{boundary, extended.Add(-time.Nanosecond)} {
		if err := store.ExpirePurchase(ctx, purchase.ID, at); err != nil {
			t.Fatal(err)
		}
		retained, err := store.PurchaseByID(ctx, purchase.ID)
		if err != nil || retained.Status != "active" || !retained.ValidUntil.Equal(extended) {
			t.Fatalf("stale worker expired extended term: %+v, %v", retained, err)
		}
	}
	var events int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE user_id=$1 AND kind=$2`, user.ID, jobpayload.UserEventExpiration).Scan(&events); err != nil || events != 0 {
		t.Fatalf("premature expiration events = %d, %v", events, err)
	}
	for range 2 {
		if err := store.ExpirePurchase(ctx, purchase.ID, extended); err != nil {
			t.Fatal(err)
		}
	}
	if expired, err := store.PurchaseByID(ctx, purchase.ID); err != nil || expired.Status != "expired" {
		t.Fatalf("due expiry = %+v, %v", expired, err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE user_id=$1 AND kind=$2`, user.ID, jobpayload.UserEventExpiration).Scan(&events); err != nil || events != 1 {
		t.Fatalf("due expiration events = %d, %v", events, err)
	}
}

func TestEntitlementTransitionSchedulerUsesExactTermBoundaries(t *testing.T) {
	t.Parallel()
	boundary := time.Date(2026, 11, 13, 16, 0, 0, 0, time.UTC)
	for _, item := range []struct {
		name, status, wantStatus string
		from, until, at          time.Duration
		expiryJobs               int
	}{
		{"live fractional active", "active", "active", -time.Hour, 900 * time.Nanosecond, 0, 0},
		{"due whole active", "active", "active", -time.Hour, 0, 900 * time.Nanosecond, 1},
		{"future fractional start", "queued", "queued", 900 * time.Nanosecond, time.Hour, 0, 0},
		{"due whole start", "queued", "activating", 0, time.Hour, 900 * time.Nanosecond, 0},
		{"live fractional queue", "queued", "activating", -time.Hour, 900 * time.Nanosecond, 0, 0},
		{"expired whole queue", "queued", "expired", -time.Hour, 0, 900 * time.Nanosecond, 0},
		{"live fractional failure", "failed", "failed", -time.Hour, 900 * time.Nanosecond, 0, 0},
	} {
		t.Run(item.name, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			combo := saveTestCombo(t, store, "Exact transition", 100, 30)
			_, purchase := createAdminWorkflowPurchase(t, store, 89112, combo, boundary.Add(-time.Hour))
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status=$1,valid_from=$2,valid_until=$3 WHERE id=$4`,
				item.status, stamp(boundary.Add(item.from)), stamp(boundary.Add(item.until)), purchase.ID); err != nil {
				t.Fatal(err)
			}
			if err := store.EnqueueDueEntitlementTransitions(ctx, boundary.Add(item.at)); err != nil {
				t.Fatal(err)
			}
			current, err := store.PurchaseByID(ctx, purchase.ID)
			if err != nil || current.Status != item.wantStatus {
				t.Fatalf("transition status = %q, %v; want %q", current.Status, err, item.wantStatus)
			}
			var jobs int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='purchase_expire'`).Scan(&jobs); err != nil || jobs != item.expiryJobs {
				t.Fatalf("expiry jobs = %d, %v; want %d", jobs, err, item.expiryJobs)
			}
		})
	}
}
