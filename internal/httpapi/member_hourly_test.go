package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestHourlyMemberViewRequiresSessionAndPublishesIndependentHourlyOffers(t *testing.T) {
	f := newMemberHourlyFixture(t)
	if w := f.request(t, "GET", "/api/v1/hourly", "", "", false); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated hourly view = %d", w.Code)
	}
	if _, err := f.server.deps.Store.SaveSquadProduct(context.Background(), database.SquadProductInput{ID: "member-hourly",
		GroupID: "member-hourly", Name: "Hidden legacy", Description: "Member description", PriceTXBMinor: 9999,
		Visible: false, UpstreamPresent: true}); err != nil {
		t.Fatal(err)
	}
	w := f.request(t, "GET", "/api/v1/hourly?userId=someone-else", "", "", true)
	if w.Code != http.StatusOK {
		t.Fatalf("member hourly view = %d: %s", w.Code, w.Body.String())
	}
	var view model.HourlyAccountView
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Squads) != 1 || view.Squads[0].Squad.Name != "Hourly hidden legacy" || view.Squads[0].Offer.HourlyPrice.Minor != "100" ||
		view.Squads[0].RequiredBalance == nil || view.Squads[0].RequiredBalance.Minor != "16800" || view.Squads[0].Ready || view.Squads[0].ReasonCode != "agent_incompatible" {
		t.Fatalf("hourly visibility/price/readiness followed legacy state: %+v", view.Squads)
	}
	if !strings.Contains(w.Body.String(), `"freeAllowanceBytes":"9007199254740993"`) || view.Balance.Minor != "9007199254740993" || view.ExtraPreference.ExtraEnabled {
		t.Fatal("hourly view lost decimal precision or enabled extras by default")
	}
	if view.Squads[0].Squad.CreatedAt.IsZero() {
		t.Fatal("synthesized squad metadata violates the public catalog shape")
	}
	for _, forbidden := range []string{`"proxyUserId"`, `"periodId"`, `"accountGeneration"`, `"credentials"`, `"activationCode"`} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Fatalf("hourly member view exposed internal field %s", forbidden)
		}
	}
	var days int
	if err := f.server.deps.Store.DB().QueryRow(`SELECT COUNT(*) FROM hourly_credit_days`).Scan(&days); err != nil || days != 0 {
		t.Fatalf("GET created daily accounting state: %d %v", days, err)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("owner hourly view is publicly cacheable")
	}
	f.server.deps.Hourly = nil
	if w := f.request(t, "GET", "/api/v1/hourly", "", "", true); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("optional legacy dependency did not fail safely: %d", w.Code)
	}
}

func TestHourlyMemberActivationStrictUnitsAndEncryptedOwnerScopedIntent(t *testing.T) {
	f := newMemberHourlyFixture(t)
	prefix := `{"groupId":"member-hourly","policyVersion":"` + f.policy.Version + `","offerRevision":`
	for _, revision := range []string{`1`, `"01"`, `"+1"`, `"1e0"`, `"0"`, `"9223372036854775808"`, `null`} {
		if w := f.request(t, "POST", "/api/v1/hourly/activate", prefix+revision+`}`, "strict-units", true); w.Code != http.StatusBadRequest {
			t.Fatalf("noncanonical revision %s accepted: %d", revision, w.Code)
		}
	}
	valid := prefix + `"1","activationCode":"member-private-code"}`
	if w := f.request(t, "POST", "/api/v1/hourly/activate", valid, "", true); w.Code != http.StatusBadRequest {
		t.Fatalf("activation without idempotency key = %d", w.Code)
	}
	if w := f.request(t, "POST", "/api/v1/hourly/activate", prefix+`"1","ownerId":"someone-else"}`, "owner-injection", true); w.Code != http.StatusBadRequest {
		t.Fatalf("member request accepted ownership override: %d", w.Code)
	}
	w := f.request(t, "POST", "/api/v1/hourly/activate", valid, "valid-activation", true)
	var receipt model.OperationReceipt
	if json.Unmarshal(w.Body.Bytes(), &receipt) != nil || w.Code != http.StatusAccepted || receipt.ID == "" || strings.Contains(w.Body.String(), "member-private-code") {
		t.Fatalf("activation did not produce a safe queued receipt: %d %s", w.Code, w.Body.String())
	}
	stored, err := f.server.deps.Store.ProviderOperationByID(context.Background(), receipt.ID)
	if err != nil || stored.OwnerUserID != f.user.ID || stored.ActorUserID != f.user.ID {
		t.Fatalf("activation intent escaped signed-in owner: %+v %v", stored.Receipt, err)
	}
	var payload string
	if err := f.server.deps.Store.DB().QueryRow(`SELECT payload FROM outbox_jobs WHERE kind='provider_operation'`).Scan(&payload); err != nil || strings.Contains(payload, "member-private-code") {
		t.Fatal("queued operation exposed plaintext activation code", err)
	}
}
