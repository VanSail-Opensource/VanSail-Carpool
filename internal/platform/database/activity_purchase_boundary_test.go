package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestActivityRewardsRespectExactPaidWindow(t *testing.T) {
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name             string
		from, until, now time.Time
		eligible         bool
	}{
		{"whole start after fractional scan", base, base.Add(time.Hour), base.Add(500 * time.Millisecond), true},
		{"future fractional start", base.Add(500 * time.Millisecond), base.Add(time.Hour), base, false},
		{"live fractional expiry", base.Add(-time.Hour), base.Add(500 * time.Millisecond), base, true},
		{"expired whole-second term", base.Add(-time.Hour), base, base.Add(time.Nanosecond), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 950024)
			combo := saveTestCombo(t, store, "reward boundary", 0, 1)
			purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "source"}, base.Add(-time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(tc.from), stamp(tc.until), purchase.ID); err != nil {
				t.Fatal(err)
			}
			if _, err = store.AdjustBalance(ctx, user.ID, 1000, "seed", "seed", base); err != nil {
				t.Fatal(err)
			}
			message, err := store.RecordGroupMessage(ctx, user.ID, -100123, 1, tc.now.Format(time.DateOnly), "UTC", 1, 10, 2, tc.now)
			if err != nil || message.Counted != tc.eligible {
				t.Fatal("message eligibility crossed boundary", message, err)
			}
			draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{Name: "Extension", Kind: "instant", Enabled: true, FeeMinor: 100, ExpectedParticipation: 1,
				Prizes: []activity.PrizeInput{{Name: "One day", ProbabilityBPS: 10000, Reward: activity.Reward{Kind: activity.RewardSubscriptionExtension, ExtensionDays: 1}}}}, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			_, err = store.PlayLuckyDraw(ctx, user.ID, draw.ID, "play", fixedActivityRandom{}, tc.now)
			current, loadErr := store.PurchaseByID(ctx, purchase.ID)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if tc.eligible {
				if err != nil || !current.ValidUntil.Equal(tc.until.Add(24*time.Hour)) || adminWorkflowBalance(t, store, user.ID) != 910 {
					t.Fatal("live reward failed", current, err)
				}
			} else if !errors.Is(err, ErrConflict) || !current.ValidUntil.Equal(tc.until) || adminWorkflowBalance(t, store, user.ID) != 1000 {
				t.Fatal("ineligible reward changed paid state", current, err)
			}
		})
	}
}

func TestExtensionShiftsOnlyExactQueuedSuccessors(t *testing.T) {
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	for _, fraction := range []time.Duration{0, 500 * time.Nanosecond} {
		t.Run(fraction.String(), func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 950025)
			combo := saveTestCombo(t, store, "queue extension", 0, 1)
			active, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "active"}, base.Add(-24*time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			next, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "next"}, base.Add(-time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			older, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "earlier"}, base.Add(-time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			boundary := base.Add(fraction)
			if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, stamp(boundary), active.ID); err != nil {
				t.Fatal(err)
			}
			nextStart := base.Add(time.Microsecond)
			earlierStart := base.Add(-time.Nanosecond)
			if fraction > 0 {
				earlierStart = base
			}
			for _, term := range []struct {
				id   string
				from time.Time
			}{{next.ID, nextStart}, {older.ID, earlierStart}} {
				if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET status='queued',valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(term.from), stamp(base.Add(24*time.Hour)), term.id); err != nil {
					t.Fatal(err)
				}
			}
			tx, err := store.DB().BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if err = applySubscriptionExtensionHoursTx(ctx, tx, user.ID, 1, "fixture", "exact-boundary", base.Add(-time.Minute)); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
			updated, err := store.PurchaseByID(ctx, next.ID)
			if err != nil || !updated.ValidFrom.Equal(nextStart.Add(time.Hour)) {
				t.Fatal("fractional successor not shifted", updated, err)
			}
			preceding, err := store.PurchaseByID(ctx, older.ID)
			if err != nil || !preceding.ValidFrom.Equal(earlierStart) {
				t.Fatal("earlier queued term moved", preceding, err)
			}
		})
	}
}
