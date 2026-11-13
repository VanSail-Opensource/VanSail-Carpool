package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestHourlyMemberCannotDisableAnotherOwnerOrGeneration(t *testing.T) {
	f := newMemberHourlyFixture(t)
	ctx := context.Background()
	other, _, err := f.server.deps.Store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 89720, FirstName: "Other"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.server.deps.Store.AdjustBalance(ctx, other.ID, 100000, "other-http-seed", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	e, err := f.server.deps.Store.PrepareHourlyActivation(ctx, database.HourlyActivationInput{OwnerID: other.ID,
		AccountGeneration: other.AccountGeneration, GroupID: "member-hourly", ActivationKey: "other-activation",
		ExpectedPolicyVersion: f.policy.Version, ExpectedOfferRevision: 1}, f.policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	body := `{"policyVersion":"` + f.policy.Version + `"}`
	if w := f.request(t, "POST", "/api/v1/hourly/enrollments/"+e.ID+"/disable", body, "foreign-disable", true); w.Code != http.StatusNotFound {
		t.Fatalf("foreign hourly enrollment could be disabled: %d %s", w.Code, w.Body.String())
	}
	stored, err := f.server.deps.Store.HourlyEnrollment(ctx, e.ID)
	if err != nil || stored.Status != model.HourlyPreparing {
		t.Fatalf("foreign request mutated enrollment: %+v %v", stored, err)
	}
	own, err := f.server.deps.Store.PrepareHourlyActivation(ctx, database.HourlyActivationInput{OwnerID: f.user.ID,
		AccountGeneration: f.user.AccountGeneration, GroupID: "member-hourly", ActivationKey: "old-generation",
		ExpectedPolicyVersion: f.policy.Version, ExpectedOfferRevision: 1}, f.policy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.server.deps.Store.DB().Exec(`UPDATE users SET account_generation=account_generation+1 WHERE id=$1`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if w := f.request(t, "POST", "/api/v1/hourly/enrollments/"+own.ID+"/disable", body, "old-generation-disable", true); w.Code != http.StatusNotFound {
		t.Fatalf("current session could mutate an older account generation: %d", w.Code)
	}
}

func TestHourlyExtraOptOutSurvivesUnreadablePolicyAndKeepsCredit(t *testing.T) {
	f := newMemberHourlyFixture(t)
	body := `{"enabled":true,"policyVersion":"` + f.policy.Version + `","acceptedPriceTXBMinor":"25"}`
	if w := f.request(t, "PUT", "/api/v1/hourly/extra-traffic", body, "", true); w.Code != http.StatusOK {
		t.Fatalf("explicit price consent failed: %d %s", w.Code, w.Body.String())
	}
	for _, invalid := range []string{`25`, `"025"`, `"2.5"`, `"9223372036854775808"`} {
		bad := `{"enabled":true,"policyVersion":"` + f.policy.Version + `","acceptedPriceTXBMinor":` + invalid + `}`
		if w := f.request(t, "PUT", "/api/v1/hourly/extra-traffic", bad, "", true); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid decimal cap %s accepted: %d", invalid, w.Code)
		}
	}
	day := time.Now().UTC().Format(time.DateOnly)
	if _, err := f.server.deps.Store.EnsureHourlyCreditDay(context.Background(), f.user.ID, f.user.AccountGeneration, day, 1000); err != nil {
		t.Fatal(err)
	}
	if _, err := f.server.deps.Store.DB().Exec(`UPDATE hourly_credit_days SET extra_funded_bytes=1000000000 WHERE owner_id=$1`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.server.deps.Store.CompareAndSwapHourlyPolicy(context.Background(), f.admin.ID, f.policy.Version, "unreadable-ciphertext"); err != nil {
		t.Fatal(err)
	}
	w := f.request(t, "PUT", "/api/v1/hourly/extra-traffic", `{"enabled":false}`, "", true)
	var preference model.HourlyExtraPreference
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &preference) != nil || preference.ExtraEnabled {
		t.Fatalf("configuration failure trapped extra opt-in: %d %s", w.Code, w.Body.String())
	}
	credit, err := f.server.deps.Store.HourlyCreditDay(context.Background(), f.user.ID, f.user.AccountGeneration, day)
	if err != nil || credit.ExtraFundedBytes != 1000000000 {
		t.Fatalf("opt-out erased funded credit: %+v %v", credit, err)
	}
}
