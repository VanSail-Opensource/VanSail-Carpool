package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/go-chi/chi/v5"
)

func TestHourlyAdministrationRequiresNativeScopeAndCurrentTerms(t *testing.T) {
	s, handler, user := testBrowserHTTP(t)
	ctx := context.Background()
	vault, err := secret.NewVault([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	s.deps.Settings = admin.NewSettingsService(s.deps.Store, vault)
	s.deps.Admin = admin.NewService(s.deps.Store, s.deps.Settings, nil, nil, nil)
	s.deps.APITokens = apitoken.New(s.deps.Store, []int64{user.TelegramID})
	router := handler.(chi.Router)
	router.Group(func(r chi.Router) {
		r.Use(s.requireSignedRequest, s.requireSession, s.requireAdmin)
		r.Get("/api/v1/admin/hourly-policy", s.adminHourlyPolicy)
		r.Put("/api/v1/admin/hourly-policy", s.adminSaveHourlyPolicy)
		r.Get("/api/v1/admin/hourly-offers", s.adminHourlyOffers)
		r.Put("/api/v1/admin/hourly-offers/{groupId}", s.adminSaveHourlyOffer)
	})
	issue := func(scope string) string {
		t.Helper()
		issued, err := s.deps.APITokens.Create(ctx, user, apitoken.CreateInput{Name: "hourly audit", Scopes: []string{scope}})
		if err != nil {
			t.Fatal(err)
		}
		return issued.Token
	}
	reader, writer, unrelated := issue("xnode:read"), issue("xnode:write"), issue("admin:write")
	call := func(method, path, token, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, "/api/v1/admin/"+path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("%s %s returned %d, want %d: %s", method, path, response.Code, want, response.Body.String())
		}
		return response
	}
	call("GET", "hourly-policy", "", "", http.StatusUnauthorized)
	call("GET", "hourly-policy", unrelated, "", http.StatusForbidden)
	call("PUT", "hourly-policy", reader, `{}`, http.StatusForbidden)
	call("PUT", "hourly-offers/missing", unrelated, `{}`, http.StatusForbidden)
	var initial model.HourlyPolicySnapshot
	if err := json.Unmarshal(call("GET", "hourly-policy", reader, "", http.StatusOK).Body.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	if initial.Policy.SalesEnabled || initial.Policy.ActivationHours != 168 || initial.Policy.CooldownSeconds != 86400 {
		t.Fatal("unconfigured hourly sales are not safely disabled")
	}
	initial.Policy.SalesEnabled = true
	initial.Policy.ReducedRateBytesPerSecond = 125000
	initial.Policy.DailyAllowanceBytes = 1000000000
	initial.Policy.ExtraGBPriceTXBMinor = 125
	body, err := json.Marshal(initial)
	if err != nil {
		t.Fatal(err)
	}
	var saved model.HourlyPolicySnapshot
	if err := json.Unmarshal(call("PUT", "hourly-policy", writer, string(body), http.StatusOK).Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Version == initial.Version || saved.Policy != initial.Policy {
		t.Fatal("successful policy update omitted CAS or changed explicit units")
	}
	call("PUT", "hourly-policy", writer, string(body), http.StatusConflict)
	saved.Policy.ActivationHours = 0
	body, err = json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	call("PUT", "hourly-policy", writer, string(body), http.StatusUnprocessableEntity)
	call("GET", "hourly-offers", reader, "", http.StatusOK)
	call("PUT", "hourly-offers/missing", writer, `{"revision":0,"enabled":true,"hourlyPriceTXBMinor":"1"}`, http.StatusBadRequest)
	call("PUT", "hourly-offers/missing", writer, `{"revision":"0","enabled":true,"hourlyPriceTXBMinor":"-1"}`, http.StatusUnprocessableEntity)
	// Complete replacements must distinguish explicit false/zero from omitted
	// or null values, especially cooldown, offer availability and pricing.
	for _, field := range []string{"salesEnabled", "dailyAllowanceBytes", "reducedRateBytesPerSecond", "extraGBPriceTXBMinor", "activationHours", "cooldownSeconds"} {
		for _, null := range []bool{false, true} {
			policyBody, err := json.Marshal(model.HourlyPolicySnapshot{Version: saved.Version, Policy: initial.Policy})
			if err != nil {
				t.Fatal(err)
			}
			var partial map[string]any
			if err := json.Unmarshal(policyBody, &partial); err != nil {
				t.Fatal(err)
			}
			policy := partial["policy"].(map[string]any)
			delete(policy, field)
			if null {
				policy[field] = nil
			}
			encoded, err := json.Marshal(partial)
			if err != nil {
				t.Fatal(err)
			}
			call("PUT", "hourly-policy", writer, string(encoded), http.StatusBadRequest)
		}
	}
	for _, input := range []string{`{}`, `{"version":null,"policy":{}}`, `{"version":"","policy":null}`} {
		call("PUT", "hourly-policy", writer, input, http.StatusBadRequest)
	}
	for _, field := range []string{"revision", "enabled", "hourlyPriceTXBMinor"} {
		for _, null := range []bool{false, true} {
			partial := map[string]any{"revision": "0", "enabled": false, "hourlyPriceTXBMinor": "0"}
			delete(partial, field)
			if null {
				partial[field] = nil
			}
			encoded, err := json.Marshal(partial)
			if err != nil {
				t.Fatal(err)
			}
			call("PUT", "hourly-offers/missing", writer, string(encoded), http.StatusBadRequest)
		}
	}
	var unchanged model.HourlyPolicySnapshot
	if err := json.Unmarshal(call("GET", "hourly-policy", reader, "", http.StatusOK).Body.Bytes(), &unchanged); err != nil || unchanged.Version != saved.Version || unchanged.Policy != initial.Policy {
		t.Fatal("rejected partial policy changed current terms", err)
	}
}
