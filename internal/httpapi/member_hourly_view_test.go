package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestHourlyViewKeepsUnavailableOwnedSquadAndCooldown(t *testing.T) {
	f := newMemberHourlyFixture(t)
	ctx := context.Background()
	input := database.HourlyActivationInput{OwnerID: f.user.ID, AccountGeneration: f.user.AccountGeneration,
		GroupID: "member-hourly", ActivationKey: "view-owned", ExpectedPolicyVersion: f.policy.Version, ExpectedOfferRevision: 1}
	e, err := f.server.deps.Store.PrepareHourlyActivation(ctx, input, f.policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.server.deps.Store.EnsureXNodeHourlyPrincipal(ctx, e.ID, "opaque-view-credentials"); err != nil {
		t.Fatal(err)
	}
	// Construct an existing paid activation. This view fixture does not emulate
	// native readiness or execute an activation worker.
	if _, err := f.server.deps.Store.DB().Exec(`UPDATE hourly_enrollments SET status='active',activated_at=clock_timestamp()-INTERVAL '1 hour',
		paid_through=(date_trunc('hour',clock_timestamp() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')+INTERVAL '1 hour' WHERE id=$1`, e.ID); err != nil {
		t.Fatal(err)
	}
	stopped, err := f.server.deps.Store.DisableHourlyEnrollment(ctx, database.HourlyDisableInput{OwnerID: f.user.ID,
		AccountGeneration: f.user.AccountGeneration, EnrollmentID: e.ID, Voluntary: true}, f.policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.server.deps.Store.SaveHourlyOffer(ctx, f.user.ID, model.HourlyOffer{GroupID: input.GroupID, Enabled: false, HourlyPriceTXBMinor: 100}, 1); err != nil {
		t.Fatal(err)
	}
	if err := f.server.deps.Store.SaveXNodeGroup(ctx, xnode.Group{ID: input.GroupID, Name: "Unavailable owned", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	w := f.request(t, "GET", "/api/v1/hourly", "", "", true)
	var view model.HourlyAccountView
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &view) != nil || len(view.Squads) != 1 || view.Squads[0].Enrollment == nil ||
		view.Squads[0].Enrollment.ID != e.ID || view.Squads[0].Ready || view.Squads[0].CooldownUntil == nil || !view.Squads[0].CooldownUntil.Equal(*stopped.CooldownUntil) {
		t.Fatalf("unavailable owned/cooldown disappeared: %d %s", w.Code, w.Body.String())
	}
}

func TestHourlyViewReportsPausedSalesAndAcceptedFreeLegacyTerm(t *testing.T) {
	f := newMemberHourlyFixture(t)
	ctx := context.Background()
	policy := f.policy.Policy
	policy.SalesEnabled = false
	if _, err := f.server.deps.Settings.SaveHourlyPolicy(ctx, f.user.ID, f.policy.Version, policy); err != nil {
		t.Fatal(err)
	}
	combo, err := f.server.deps.Store.SaveCombo(ctx, database.ComboInput{Name: "Historical free term", ValidityDays: 30,
		TrafficLimitBytes: 1000, ResetStrategy: "DAY", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.server.deps.Store.CreatePurchase(ctx, database.PurchaseInput{UserID: f.user.ID, ComboID: combo.ID, IdempotencyKey: "free-legacy"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	w := f.request(t, "GET", "/api/v1/hourly", "", "", true)
	var view model.HourlyAccountView
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &view) != nil || view.Policy.Policy.SalesEnabled || !view.LegacyBlocked || view.Squads[0].Ready {
		t.Fatalf("free legacy term/paused hourly sales were misrepresented: %d %s", w.Code, w.Body.String())
	}
}
