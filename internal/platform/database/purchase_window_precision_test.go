package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
)

func TestPurchaseEligibilityUsesExactFractionalBoundaries(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 970021)
	combo := saveTestCombo(t, store, "fractional-window", 0, 30)
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: owner.ID, ComboID: combo.ID,
		IdempotencyKey: "fractional-window"}, base.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, status       string
		from, until, now   time.Time
		desired, community bool
	}{
		{"whole-start", "active", base, base.Add(time.Second), base.Add(500 * time.Millisecond), true, true},
		{"future-fraction", "active", base.Add(500 * time.Millisecond), base.Add(time.Second), base, false, false},
		{"fractional-expiry", "active", base.Add(-time.Second), base.Add(500 * time.Millisecond), base, true, true},
		{"expired-whole-second", "active", base.Add(-time.Second), base, base.Add(time.Nanosecond), false, false},
		{"before-nanosecond-expiry", "active", base.Add(-time.Second), base.Add(900 * time.Nanosecond), base.Add(899 * time.Nanosecond), true, true},
		{"exact-nanosecond-expiry", "active", base.Add(-time.Second), base.Add(900 * time.Nanosecond), base.Add(900 * time.Nanosecond), false, false},
		{"before-nanosecond-start", "active", base.Add(700 * time.Nanosecond), base.Add(time.Second), base.Add(699 * time.Nanosecond), false, false},
		{"activating", "activating", base, base.Add(time.Second), base.Add(500 * time.Millisecond), true, false},
		{"expired-status", "expired", base, base.Add(time.Second), base.Add(500 * time.Millisecond), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status=$1,valid_from=$2,valid_until=$3 WHERE id=$4`,
				tc.status, stamp(tc.from), stamp(tc.until), purchase.ID); err != nil {
				t.Fatal(err)
			}
			eligible, err := store.HasActiveCombo(ctx, owner.ID, tc.now)
			if err != nil || eligible != tc.community {
				t.Fatal("community eligibility crossed exact boundary", eligible, err)
			}
			desired, err := store.DesiredEntitlement(ctx, owner.ID, tc.now)
			if err != nil || (desired != nil) != tc.desired {
				t.Fatal("desired entitlement crossed exact boundary", desired, err)
			}
			active, _, err := store.ActiveAndQueuedPurchases(ctx, owner.ID, tc.now)
			if err != nil || (active != nil) != tc.desired {
				t.Fatal("home entitlement crossed exact boundary", active, err)
			}
			enabled := true
			snapshot, err := store.UpdateUserPreferences(ctx, owner.ID,
				preferences.Patch{ShowAroundTX: &enabled, ShowActivity: &enabled}, tc.now)
			if err != nil || snapshot.ActiveCombo != tc.community ||
				snapshot.ShowAroundTX != tc.community || snapshot.ShowActivity != tc.community {
				t.Fatal("preference write used different eligibility", snapshot, err)
			}
		})
	}
}

func TestOverlappingHistoricalPurchasesChooseParsedNewestStart(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 970022)
	combo := saveTestCombo(t, store, "fractional-newest", 0, 30)
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	first, err := store.CreatePurchase(ctx, PurchaseInput{UserID: owner.ID, ComboID: combo.ID,
		IdempotencyKey: "older-whole-start"}, base.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreatePurchase(ctx, PurchaseInput{UserID: owner.ID, ComboID: combo.ID,
		IdempotencyKey: "newer-fractional-start"}, base.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	// Retained administrative history can contain overlapping windows. Both
	// projections must choose the same actual newest start, not text order.
	for id, start := range map[string]time.Time{first.ID: base, second.ID: base.Add(500 * time.Millisecond)} {
		if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='active',valid_from=$1,valid_until=$2 WHERE id=$3`,
			stamp(start), stamp(base.Add(time.Hour)), id); err != nil {
			t.Fatal(err)
		}
	}
	history, err := store.ListPurchases(ctx, owner.ID)
	if err != nil || len(history) != 2 || history[0].ID != second.ID {
		t.Fatal("member history selected lexical start", history, err)
	}
	preloaded, err := store.XNodeCompileMembers(ctx, []string{owner.ID})
	if err != nil || len(preloaded.Purchases[owner.ID]) != 2 || preloaded.Purchases[owner.ID][0].ID != second.ID {
		t.Fatal("native preload selected lexical start", preloaded.Purchases[owner.ID], err)
	}
	now := base.Add(time.Second)
	desired, err := store.DesiredEntitlement(ctx, owner.ID, now)
	if err != nil || desired == nil || desired.ID != second.ID {
		t.Fatal("desired entitlement selected lexical start", desired, err)
	}
	active, _, err := store.ActiveAndQueuedPurchases(ctx, owner.ID, now)
	if err != nil || active == nil || active.ID != second.ID {
		t.Fatal("home entitlement selected lexical start", active, err)
	}
}
