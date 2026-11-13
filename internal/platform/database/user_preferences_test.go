package database

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
	"testing"
	"time"
)

func TestUserPreferencesDefaultsMergeAndEligibility(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := createTestUser(t, s, 55101)
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	p, err := s.UserPreferences(ctx, u.ID)
	if err != nil || p != preferences.Defaults() {
		t.Fatalf("defaults: %+v %v", p, err)
	}
	on, off := true, false
	got, err := s.UpdateUserPreferences(ctx, u.ID, preferences.Patch{ShowActivity: &on, ShowAroundTX: &on, Notifications: &preferences.NotificationPatch{Money: &off}}, now)
	if err != nil || got.ShowActivity || got.ShowAroundTX || got.Notifications.Money {
		t.Fatalf("gate: %+v %v", got, err)
	}
	got, err = s.UpdateUserPreferences(ctx, u.ID, preferences.Patch{IncludeNodePrices: &off}, now)
	if err != nil || got.Notifications.Money || got.IncludeNodePrices || !got.Notifications.Combos {
		t.Fatalf("merge: %+v %v", got, err)
	}
	other := createTestUser(t, s, 55102)
	untouched, err := s.UserPreferences(ctx, other.ID)
	if err != nil || untouched != preferences.Defaults() {
		t.Fatalf("other user: %+v %v", untouched, err)
	}
}

func TestPaidAroundEntranceReturnsAfterLaterActivation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := createTestUser(t, s, 55103)
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, s, "preferences-window", 100, 30)
	if _, err := s.AdjustBalance(ctx, u.ID, 500, "preferences-seed", "credit", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := s.CreatePurchase(ctx, PurchaseInput{UserID: u.ID, ComboID: combo.ID, IdempotencyKey: "preferences-first"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE purchases SET status='active',updated_at=$1 WHERE id=$2`, stamp(now), purchase.ID); err != nil {
		t.Fatal(err)
	}
	on := true
	got, err := s.UpdateUserPreferences(ctx, u.ID, preferences.Patch{ShowActivity: &on, ShowAroundTX: &on}, now)
	if err != nil || !got.ShowActivity || !got.ShowAroundTX {
		t.Fatalf("opt in: %+v %v", got, err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE purchases SET status='cancelled',updated_at=$1 WHERE id=$2`, stamp(now.Add(time.Hour)), purchase.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE purchases SET status='active',valid_until=$1,updated_at=$2 WHERE id=$3`, stamp(now.Add(48*time.Hour)), stamp(now.Add(2*time.Hour)), purchase.ID); err != nil {
		t.Fatal(err)
	}
	got, err = s.UserPreferenceSnapshot(ctx, u.ID, now.Add(3*time.Hour))
	if err != nil || !got.ActiveCombo || got.ShowActivity || !got.ShowAroundTX {
		t.Fatalf("later activation: %+v %v", got, err)
	}
}

func TestReferralPreferenceHidesOnlyAttribution(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	u := createTestUser(t, s, 55104)
	now := time.Now().UTC()
	off := false
	if _, err := s.UpdateUserPreferences(ctx, u.ID, preferences.Patch{ShowReferralUsername: &off}, now); err != nil {
		t.Fatal(err)
	}
	name, accepted, err := s.AcceptAffiliateReferral(ctx, 55105, u.TelegramID, now)
	if err != nil || !accepted || name != "" {
		t.Fatalf("referral: %q %t %v", name, accepted, err)
	}
	invitee, err := s.UserByTelegramID(ctx, 55105)
	if err != nil || invitee.InviterID == nil || *invitee.InviterID != u.TelegramID {
		t.Fatalf("tracking: %+v %v", invitee, err)
	}
}

func TestPreferenceMutationCannotRecreateClosedAccountState(t *testing.T) {
	for _, status := range []string{"purging", "purged"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 55106)
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status=$1 WHERE id=$2`, status, user.ID); err != nil {
				t.Fatal(err)
			}
			off := false
			if _, err := store.UpdateUserPreferences(ctx, user.ID, preferences.Patch{PAYGEnabled: &off, ShowReferralUsername: &off}, time.Now().UTC()); !errors.Is(err, ErrAccountUnavailable) {
				t.Fatal("closed owner changed preferences", err)
			}
			var preferences, jobs int
			if err := store.DB().QueryRowContext(ctx, `SELECT
				(SELECT COUNT(*) FROM user_preferences WHERE user_id=$1),
				(SELECT COUNT(*) FROM outbox_jobs WHERE payload::jsonb->>'userId'=$1)`, user.ID).Scan(&preferences, &jobs); err != nil || preferences != 0 || jobs != 0 {
				t.Fatal("closed owner gained new preference or repair state", preferences, jobs, err)
			}
		})
	}
}
