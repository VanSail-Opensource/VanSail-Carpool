package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestUsageBreakdownHTTPUsesOnlySessionOwnerAndPreservesExactBytes(t *testing.T) {
	f := newUsageBreakdownHTTPFixture(t, false)
	other, _, err := f.server.deps.Store.UpsertTelegramUser(context.Background(), model.TelegramProfile{ID: 80306}, false)
	if err != nil {
		t.Fatal(err)
	}
	seedUsageBreakdownHTTPRow(t, f, f.user.ID, "owned-retired-node", "9007199254740993")
	seedUsageBreakdownHTTPRow(t, f, other.ID, "foreign-private-node", "999")
	w := f.request(t, "GET", "/api/v1/usage/breakdown?ownerId="+other.ID, "", "", true)
	var result model.UsageBreakdown
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Nodes) != 1 || result.Nodes[0].NodeID != "owned-retired-node" || result.Nodes[0].UploadBytes != "9007199254740993" || result.PositiveNodes.Traffic != "1" {
		t.Fatalf("owner-scoped exact projection failed: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "foreign-private-node") || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("private history leaked or became cacheable", w.Header())
	}
}

func TestUsageBreakdownHTTPRequiresAuthenticationAndOnboarding(t *testing.T) {
	f := newUsageBreakdownHTTPFixture(t, false)
	if w := f.request(t, "GET", "/api/v1/usage/breakdown", "", "", false); w.Code != http.StatusUnauthorized {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := f.server.deps.Store.DB().ExecContext(context.Background(), `UPDATE users SET onboarding_state='intro' WHERE id=$1`, f.user.ID); err != nil {
		t.Fatal(err)
	}
	if w := f.request(t, "GET", "/api/v1/usage/breakdown", "", "", true); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "ONBOARDING_REQUIRED") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestUsageBreakdownHTTPDoesNotFakeUnavailableHistory(t *testing.T) {
	f := newUsageBreakdownHTTPFixture(t, false)
	f.server.deps.XNode = xnode.NewService(nil, nil, nil, nil, nil)
	if w := f.request(t, "GET", "/api/v1/usage/breakdown", "", "", true); w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "XNODE_UNAVAILABLE") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestUsageBreakdownMemberRouteDoesNotWidenAdministratorBearerScopes(t *testing.T) {
	f := newUsageBreakdownHTTPFixture(t, true)
	f.server.deps.APITokens = apitoken.New(f.server.deps.Store, []int64{f.user.TelegramID})
	for _, scopes := range [][]string{{"admin:write"}, {"xnode:write"}, {"admin:write", "xnode:write"}} {
		issued, err := f.server.deps.APITokens.Create(context.Background(), f.user, apitoken.CreateInput{Name: "member privacy", Scopes: scopes})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("GET", "/api/v1/usage/breakdown", nil)
		r.Header.Set("Authorization", "Bearer "+issued.Token)
		w := httptest.NewRecorder()
		f.router.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "API_TOKEN_SCOPE_REQUIRED") {
			t.Fatalf("scopes %v bypassed member ownership: %d %s", scopes, w.Code, w.Body.String())
		}
	}
}
