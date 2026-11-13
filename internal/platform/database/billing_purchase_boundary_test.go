package database

import (
	"context"
	"testing"
	"time"
)

func TestPurchaseQueueStartsAfterLatestExactPaidBoundary(t *testing.T) {
	t.Parallel()
	for _, fraction := range []time.Duration{900 * time.Nanosecond, 500 * time.Millisecond} {
		t.Run(fraction.String(), func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
			user := createTestUser(t, store, 69_030)
			combo := saveTestCombo(t, store, "exact-queue-boundary", 100, 30)
			if _, err := store.AdjustBalance(ctx, user.ID, 1000, "queue-seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			first, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "first"}, now)
			if err != nil {
				t.Fatal(err)
			}
			second, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "second"}, now)
			if err != nil {
				t.Fatal(err)
			}
			want := first.ValidUntil.Add(fraction)
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, stamp(want), second.ID); err != nil {
				t.Fatal(err)
			}
			quote, err := store.QuotePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID}, now)
			if err != nil || !quote.EffectiveAt.Equal(want) {
				t.Fatalf("next purchase starts at %v, %v; want exact latest end %v", quote.EffectiveAt, err, want)
			}
		})
	}
}
