package database

import (
	"context"
	"testing"
	"time"
)

func TestCommunityMembershipPersistsFactsWithoutOnboardingTransition(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	user := createTestUser(t, store, 22003)
	updated, err := store.UpdateMembership(context.Background(), user.ID, true, true)
	if err != nil || updated.OnboardingState != "intro" || !updated.GroupJoined || !updated.ChannelJoined {
		t.Fatalf("UpdateMembership() = (%+v, %v)", updated, err)
	}
}

func TestHasActiveComboUsesStrictCurrentActiveWindow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 22004)
	combo := saveTestCombo(t, store, "community-window", 100, 30)
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "community-window", "test credit", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "community-window"}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		status string
		from   time.Time
		until  time.Time
		want   bool
	}{
		{name: "active at inclusive start", status: "active", from: now, until: now.Add(time.Hour), want: true},
		{name: "activating denied", status: "activating", from: now.Add(-time.Hour), until: now.Add(time.Hour)},
		{name: "queued denied", status: "queued", from: now.Add(-time.Hour), until: now.Add(time.Hour)},
		{name: "future denied", status: "active", from: now.Add(time.Second), until: now.Add(time.Hour)},
		{name: "expiry boundary denied", status: "active", from: now.Add(-time.Hour), until: now},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status=$1,valid_from=$2,valid_until=$3 WHERE id=$4`, test.status, stamp(test.from), stamp(test.until), purchase.ID); err != nil {
				t.Fatal(err)
			}
			got, err := store.HasActiveCombo(ctx, user.ID, now)
			if err != nil || got != test.want {
				t.Fatalf("HasActiveCombo() = (%t, %v), want %t", got, err, test.want)
			}
		})
	}
}
