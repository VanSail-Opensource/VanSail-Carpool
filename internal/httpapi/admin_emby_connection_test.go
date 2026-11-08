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
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/go-chi/chi/v5"
)

func TestEmbyConnectionHTTPIsPrivateAtomicAndScoped(t *testing.T) {
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
		r.Get("/api/v1/admin/emby-connection", s.adminEmbyConnection)
		r.Put("/api/v1/admin/emby-connection", s.adminSaveEmbyConnection)
		r.Get("/api/v1/admin/settings", s.adminSettings)
		r.Post("/api/v1/admin/settings", s.adminCreateSetting)
		r.Put("/api/v1/admin/settings/{key}", s.adminUpdateSetting)
	})
	issue := func(scope string) string {
		t.Helper()
		issued, err := s.deps.APITokens.Create(ctx, user, apitoken.CreateInput{Name: "Emby settings", Scopes: []string{scope}})
		if err != nil {
			t.Fatal(err)
		}
		return issued.Token
	}
	reader, writer := issue("admin:read"), issue("admin:write")
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
			t.Fatalf("%s %s: %d want %d", method, path, response.Code, want)
		}
		if strings.Contains(response.Body.String(), "private-emby.example") || strings.Contains(response.Body.String(), "private-token") {
			t.Fatal("connection secret returned in response")
		}
		return response
	}
	call("GET", "emby-connection", "", "", http.StatusUnauthorized)
	call("PUT", "emby-connection", reader, `{}`, http.StatusForbidden)
	var view admin.EmbyConnectionView
	read := call("GET", "emby-connection", reader, "", http.StatusOK)
	if err := json.Unmarshal(read.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	initial := view.Version
	body := `{"version":"` + initial + `","baseURL":"https://private-emby.example","apiToken":"private-token"}`
	saved := call("PUT", "emby-connection", writer, body, http.StatusOK)
	if err := json.Unmarshal(saved.Body.Bytes(), &view); err != nil || !view.URLConfigured || !view.TokenConfigured {
		t.Fatal("connection presence missing", err)
	}
	call("GET", "settings", reader, "", http.StatusOK)
	call("PUT", "settings/emby.base_url", writer, `{"value":"https://attacker.example"}`, http.StatusUnprocessableEntity)
	call("POST", "settings", writer, `{"key":"emby.api_token","value":"injected-token"}`, http.StatusUnprocessableEntity)
	call("PUT", "emby-connection", writer, `{"version":"`+view.Version+`","baseURL":"https://attacker.example","apiToken":""}`, http.StatusUnprocessableEntity)
	call("PUT", "emby-connection", writer, body, http.StatusConflict)
	baseURL, token, err := s.deps.Settings.EmbyCredentials(ctx)
	if err != nil || baseURL != "https://private-emby.example" || token != "private-token" {
		t.Fatal("rejected request changed runtime credentials", err)
	}
}
