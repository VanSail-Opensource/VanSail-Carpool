package database

import (
	"context"
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

func TestPreferenceEntrancesStayOffAfterLaterActivation(t *testing.T) {
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
	if _, err := s.DB().ExecContext(ctx, `UPDATE purchases SET status='active',updated_at=? WHERE id=?`, stamp(now), purchase.ID); err != nil {
		t.Fatal(err)
	}
	on := true
	got, err := s.UpdateUserPreferences(ctx, u.ID, preferences.Patch{ShowActivity: &on, ShowAroundTX: &on}, now)
	if err != nil || !got.ShowActivity || !got.ShowAroundTX {
		t.Fatalf("opt in: %+v %v", got, err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE purchases SET status='cancelled',updated_at=? WHERE id=?`, stamp(now.Add(time.Hour)), purchase.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE purchases SET status='active',valid_until=?,updated_at=? WHERE id=?`, stamp(now.Add(48*time.Hour)), stamp(now.Add(2*time.Hour)), purchase.ID); err != nil {
		t.Fatal(err)
	}
	got, err = s.UserPreferenceSnapshot(ctx, u.ID, now.Add(3*time.Hour))
	if err != nil || !got.ActiveCombo || got.ShowActivity || got.ShowAroundTX {
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
