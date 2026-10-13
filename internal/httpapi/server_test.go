package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

func TestDecodeJSONAcceptsOneStrictDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "valid object", body: `{"name":"Ada"}`},
		{name: "empty body", body: ``, wantErr: true},
		{name: "unknown field", body: `{"name":"Ada","role":"admin"}`, wantErr: true},
		{name: "printable password", body: `{"name":" 密碼 päss 🧩 "}`},
		{name: "control character", body: `{"name":"Ada\u0000Lovelace"}`, wantErr: true},
		{name: "second document", body: `{"name":"Ada"} {"name":"Grace"}`, wantErr: true},
		{name: "trailing invalid bytes", body: `{"name":"Ada"} trailing`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest("POST", "/", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			var input struct {
				Name string `json:"name"`
			}
			err := decodeJSON(response, request, &input)
			if test.wantErr && err == nil {
				t.Fatal("decodeJSON() unexpectedly succeeded")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("decodeJSON(): %v", err)
			}
		})
	}
}

func TestRequireOnboardedKeepsUnonboardedAdminsOutOfProductAPIs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		user       model.User
		wantAccess bool
	}{
		{name: "unonboarded admin", user: model.User{Role: "admin", OnboardingState: "intro"}},
		{name: "unonboarded user", user: model.User{Role: "user", OnboardingState: "agreement"}},
		{name: "completed admin", user: model.User{Role: "admin", OnboardingState: "complete"}, wantAccess: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
			allowed := (&Server{}).requireOnboarded(response, request, test.user)
			if allowed != test.wantAccess {
				t.Fatalf("requireOnboarded() = %t, want %t", allowed, test.wantAccess)
			}
			if test.wantAccess {
				return
			}
			if response.Code != http.StatusConflict {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
			}
			var body apiError
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if body.Code != "ONBOARDING_REQUIRED" {
				t.Fatalf("error code = %q, want ONBOARDING_REQUIRED", body.Code)
			}
		})
	}
}

func TestSPAHandlerServesAssetsAndFallsBackToIndex(t *testing.T) {
	t.Parallel()

	content := fstest.MapFS{
		"index.html":    {Data: []byte("<main>TX Carpool</main>")},
		"admin.html":    {Data: []byte("<main>TX Carpool Console</main>")},
		"assets/app.js": {Data: []byte("window.txc = true")},
	}
	handler := spaHandler(content)

	tests := []struct {
		name        string
		path        string
		wantBody    string
		contentType string
		cache       string
	}{
		{name: "index", path: "/", wantBody: "<main>TX Carpool</main>", contentType: "text/html", cache: "no-cache, must-revalidate"},
		{name: "static asset", path: "/assets/app.js", wantBody: "window.txc = true", contentType: "text/javascript", cache: "public, max-age=31536000, immutable"},
		{name: "client route", path: "/catalog/checkout", wantBody: "<main>TX Carpool</main>", contentType: "text/html", cache: "no-cache, must-revalidate"},
		{name: "admin root", path: "/admin", wantBody: "<main>TX Carpool Console</main>", contentType: "text/html", cache: "no-cache, must-revalidate"},
		{name: "admin deep link", path: "/admin/users/user-id", wantBody: "<main>TX Carpool Console</main>", contentType: "text/html", cache: "no-cache, must-revalidate"},
		{name: "admin entry", path: "/admin.html", wantBody: "<main>TX Carpool Console</main>", contentType: "text/html", cache: "no-cache, must-revalidate"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest("GET", test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != 200 {
				t.Fatalf("status = %d, want 200", response.Code)
			}
			if got := strings.TrimSpace(response.Body.String()); got != test.wantBody {
				t.Fatalf("body = %q, want %q", got, test.wantBody)
			}
			if got := response.Header().Get("Content-Type"); !strings.Contains(got, test.contentType) {
				t.Fatalf("Content-Type = %q, want it to contain %q", got, test.contentType)
			}
			if got := response.Header().Get("Cache-Control"); got != test.cache {
				t.Fatalf("Cache-Control = %q, want %q", got, test.cache)
			}
		})
	}
}
