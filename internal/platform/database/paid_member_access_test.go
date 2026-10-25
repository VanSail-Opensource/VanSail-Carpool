package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPaidPAYGGrantsMemberAccessAndPreservesPreferences(t *testing.T) {
	s, parent, _, a := paygBillingFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if allowed, err := s.HasActiveCombo(ctx, parent.UserID, now); err != nil || allowed {
		t.Fatal("unbilled account qualified", err)
	}
	paygReport(t, s, a, 1, uint64(xnode.PAYGStartBytes))
	got, err := s.UserPreferenceSnapshot(ctx, parent.UserID, now)
	if err != nil || !got.ActiveCombo || !got.ShowAroundTX {
		t.Fatal("paid member denied", got, err)
	}
	on, off := true, false
	got, err = s.UpdateUserPreferences(ctx, parent.UserID, preferences.Patch{ShowActivity: &on, ShowAroundTX: &off}, now)
	if err != nil || !got.ActiveCombo || !got.ShowActivity || got.ShowAroundTX {
		t.Fatal("paid member choices lost", got, err)
	}
	// A combo lifecycle change cannot revoke a PAYG spender's community choices.
	combo := saveTestCombo(t, s, "payg-member", 100, 30)
	p, err := s.CreatePurchase(ctx, PurchaseInput{UserID: parent.UserID, ComboID: combo.ID, IdempotencyKey: "paid-member"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE purchases SET status='cancelled',updated_at=? WHERE id=?`, stamp(now), p.ID); err != nil {
		t.Fatal(err)
	}
	got, err = s.UserPreferenceSnapshot(ctx, parent.UserID, now.Add(96*time.Hour))
	if err != nil || !got.ActiveCombo || !got.ShowAroundTX || !got.ShowActivity {
		t.Fatal("combo revoked paid PAYG membership", got, err)
	}
}
