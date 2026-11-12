package database

import (
	"context"
	"testing"
	"time"
)

func TestPurchaseExpiryPreservesExactPaidBoundaries(t *testing.T) {
	t.Parallel()
	boundary := time.Date(2026, 11, 12, 14, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name                         string
		expires, nextStarts, at      time.Duration
		wantStatus, wantQueuedStatus string
	}{
		{"live fractional term", 900 * time.Nanosecond, 900 * time.Nanosecond, 0, "activating", "queued"},
		{"future fractional successor", 0, 900 * time.Nanosecond, 0, "expired", "queued"},
		{"due whole-second successor", 0, 0, 900 * time.Nanosecond, "expired", "activating"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 69_031)
			combo := saveTestCombo(t, store, "expiry-boundary", 100, 1)
			created := boundary.Add(-24 * time.Hour)
			if _, err := store.AdjustBalance(ctx, user.ID, 1000, "seed", "seed", created); err != nil {
				t.Fatal(err)
			}
			first, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "first"}, created)
			if err != nil {
				t.Fatal(err)
			}
			second, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "second"}, created)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1,auto_renew_enabled=0 WHERE id=$2`, stamp(boundary.Add(test.expires)), first.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_from=$1 WHERE id=$2`, stamp(boundary.Add(test.nextStarts)), second.ID); err != nil {
				t.Fatal(err)
			}
			at := boundary.Add(test.at)
			if err := store.ExpireDuePurchase(ctx, first.ID, at); err != nil {
				t.Fatal(err)
			}
			for _, check := range []struct{ id, status string }{{first.ID, test.wantStatus}, {second.ID, test.wantQueuedStatus}} {
				purchase, err := store.PurchaseByID(ctx, check.id)
				if err != nil || purchase.Status != check.status {
					t.Fatalf("purchase %s status = %q, %v; want %q", check.id, purchase.Status, err, check.status)
				}
			}
			if balance, err := store.Balance(ctx, user.ID); err != nil || balance.MinorInt64() != 800 {
				t.Fatalf("expiry changed wallet: %+v, %v", balance, err)
			}
		})
	}
}
