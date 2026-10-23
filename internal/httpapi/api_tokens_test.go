package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/go-chi/chi/v5"
)

func TestBearerTokenScopesRevocationAndSessionIsolation(t *testing.T) {
	s, handler, user := testBrowserHTTP(t)
	s.deps.APITokens = apitoken.New(s.deps.Store, []int64{user.TelegramID})
	router := handler.(chi.Router)
	router.Group(func(r chi.Router) {
		r.Use(s.requireSignedRequest, s.requireSession, s.requireAdmin)
		r.Get("/api/v1/admin/xnode/nodes", func(w http.ResponseWriter, r *http.Request) {
			if currentUser(r).ID != user.ID {
				t.Error("wrong API actor")
			}
			w.WriteHeader(204)
		})
		r.Post("/api/v1/admin/xnode/nodes", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
		r.Get("/api/v1/admin/settings", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
		r.Get("/api/v1/admin/xnode/groups", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
		r.Get("/api/v1/admin/api-tokens", s.adminAPITokens)
		r.Get("/api/v1/me", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	})
	issued, err := s.deps.APITokens.Create(context.Background(), user, apitoken.CreateInput{Name: "monitor", Scopes: []string{"xnode:read"}})
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, token string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if len(w.Result().Cookies()) != 0 {
			t.Fatal("API authentication created browser cookies")
		}
		return w
	}
	for _, path := range []string{"/api/v1/admin/xnode/nodes", "/api/v1/admin/xnode/groups"} {
		if w := request("GET", path, issued.Token); w.Code != 204 {
			t.Fatalf("allowed %s=%d:%s", path, w.Code, w.Body.String())
		}
	}
	for _, entry := range [][2]string{{"POST", "/api/v1/admin/xnode/nodes"}, {"GET", "/api/v1/admin/settings"}, {"GET", "/api/v1/admin/api-tokens"}, {"GET", "/api/v1/me"}} {
		if w := request(entry[0], entry[1], issued.Token); w.Code != 403 {
			t.Fatalf("scope escaped %v=%d", entry, w.Code)
		}
	}
	if err = s.deps.APITokens.Revoke(context.Background(), user, issued.Item.ID); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/api/v1/admin/xnode/nodes", issued.Token); w.Code != 401 {
		t.Fatalf("revoked token accepted=%d", w.Code)
	}
	issued, err = s.deps.APITokens.Create(context.Background(), user, apitoken.CreateInput{Name: "role", Scopes: []string{"admin:read"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.deps.Store.DB().ExecContext(context.Background(), `UPDATE users SET role='user' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/api/v1/admin/xnode/nodes", issued.Token); w.Code != 401 {
		t.Fatalf("demoted owner retained access=%d", w.Code)
	}
}

func TestAPITokenBrowserManagementShowsSecretOnce(t *testing.T) {
	s, handler, user := testBrowserHTTP(t)
	s.deps.APITokens = apitoken.New(s.deps.Store, []int64{user.TelegramID})
	router := handler.(chi.Router)
	router.Group(func(r chi.Router) {
		r.Use(s.requireSignedRequest, s.requireSession, s.requireAdmin)
		r.Post("/api/v1/admin/api-tokens", s.adminCreateAPIToken)
		r.Get("/api/v1/admin/api-tokens", s.adminAPITokens)
	})
	_, browser, _, err := createBrowserTestSession(t, s, user)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"name":"assistant","scopes":["xnode:write"],"expiresInDays":7}`
	r := httptest.NewRequest("POST", "/api/v1/admin/api-tokens", strings.NewReader(body))
	signBrowserTestRequest(t, s, r, browser, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	var issued apitoken.Issued
	if json.Unmarshal(w.Body.Bytes(), &issued) != nil || w.Code != 201 || len(issued.Token) != 47 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create=%d:%s", w.Code, w.Body.String())
	}
	r = httptest.NewRequest("GET", "/api/v1/admin/api-tokens", nil)
	signBrowserTestRequest(t, s, r, browser, "")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != 200 || strings.Contains(w.Body.String(), issued.Token) || strings.Contains(w.Body.String(), `"hash"`) {
		t.Fatalf("secret leaked in metadata=%d:%s", w.Code, w.Body.String())
	}
	// An explicit invalid Authorization header must not fall back to cookies.
	r = httptest.NewRequest("GET", "/api/v1/admin/api-tokens", nil)
	signBrowserTestRequest(t, s, r, browser, "")
	r.Header.Set("Authorization", "Bearer invalid")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("invalid bearer fell back to browser=%d", w.Code)
	}
}

func TestAPIReceiptScopeDoesNotExposeOtherOperations(t *testing.T) {
	s, handler, user := testBrowserHTTP(t)
	s.deps.APITokens = apitoken.New(s.deps.Store, []int64{user.TelegramID})
	router := handler.(chi.Router)
	router.With(s.requireSignedRequest, s.requireSession, s.requireAdmin).Get("/api/v1/admin/operations/{operationId}", s.adminOperationReceipt)
	issued, err := s.deps.APITokens.Create(context.Background(), user, apitoken.CreateInput{Name: "migration", Scopes: []string{"xnode:write"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"xnode_migration", "xnode_command", "refund"} {
		op, _, err := s.deps.Store.CreateProviderOperation(context.Background(), providerops.CreateInput{ActorUserID: user.ID, Kind: kind, IdempotencyKey: kind, RequestFingerprint: strings.Repeat("f", 64), SealedTarget: "private"}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("GET", "/api/v1/admin/operations/"+op.Receipt.ID, nil)
		r.Header.Set("Authorization", "Bearer "+issued.Token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		want := 403
		if kind == "xnode_migration" {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("receipt %s=%d:%s", kind, w.Code, w.Body.String())
		}
	}
}
