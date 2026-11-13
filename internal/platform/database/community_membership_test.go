package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCommunityMembershipUpdatesOnlyObservedSpace(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 22005)
	for _, step := range []struct {
		space                  string
		joined, group, channel bool
	}{
		{space: "group", joined: true, group: true},
		{space: "channel", joined: true, group: true, channel: true},
		{space: "group", joined: false, channel: true},
		{space: "channel", joined: false},
	} {
		updated, err := store.UpdateCommunityMembership(ctx, user.ID, step.space, step.joined)
		if err != nil || updated.GroupJoined != step.group || updated.ChannelJoined != step.channel || updated.OnboardingState != "intro" {
			t.Fatalf("UpdateCommunityMembership(%q, %t) = (%+v, %v)", step.space, step.joined, updated, err)
		}
	}
	if _, err := store.UpdateCommunityMembership(ctx, user.ID, "group_joined", true); !errors.Is(err, ErrConflict) {
		t.Fatalf("unrecognized space = %v, want conflict", err)
	}
	current, err := store.UserByID(ctx, user.ID)
	if err != nil || current.GroupJoined || current.ChannelJoined || current.OnboardingState != "intro" {
		t.Fatalf("invalid space changed membership: (%+v, %v)", current, err)
	}
	if _, err := store.UpdateCommunityMembership(ctx, "missing-user", "group", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user = %v, want not found", err)
	}
}

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
