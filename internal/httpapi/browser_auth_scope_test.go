package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/requestauth"
	"github.com/go-chi/chi/v5"
)

func TestBrowserHTTPAdminScopeReplayAndRoleRevocation(t *testing.T) {
	server, handler, user := testBrowserHTTP(t)
	router := handler.(chi.Router)
	router.With(server.requireSignedRequest, server.requireSession, server.requireAdmin).Get("/api/v1/admin/auth-fixture", func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r).ID != user.ID {
			t.Error("wrong admin identity")
		}
		w.WriteHeader(204)
	})
	server.deps.Accounts = accounts.NewService(server.deps.Store, nil, nil, nil, nil, nil, []int64{user.TelegramID}, 7*24*time.Hour)
	_, token, _, err := createBrowserTestSession(t, server, user)
	if err != nil {
		t.Fatal(err)
	}
	mini, err := ids.Token(32)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(mini))
	if err = server.deps.Store.ReplaceSession(context.Background(), hash[:], user.ID, time.Now().Add(7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	miniRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth-fixture", nil)
	signBrowserTestRequest(t, server, miniRequest, mini, "")
	// Move the valid Mini App bearer/signature to its actual cookie names.
	cookies := miniRequest.Cookies()
	miniRequest.Header.Del("Cookie")
	for _, cookie := range cookies {
		if cookie.Name == browserSessionCookie {
			cookie.Name = sessionCookie
		} else if cookie.Name == browserRequestKeyCookie {
			cookie.Name = requestauth.ClientKeyCookie
		}
		miniRequest.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, miniRequest)
	if response.Code != 401 {
		t.Fatalf("Mini App authenticated browser admin=%d", response.Code)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth-fixture", nil)
	signBrowserTestRequest(t, server, request, token, "")
	replay := request.Clone(request.Context())
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 204 {
		t.Fatalf("browser admin rejected=%d:%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, replay)
	if response.Code != 409 {
		t.Fatalf("signed admin replay accepted=%d", response.Code)
	}
	if _, err = server.deps.Store.DB().ExecContext(context.Background(), `UPDATE users SET role='user' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/browser/session", nil)
	signBrowserTestRequest(t, server, request, token, "")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var state struct {
		Kind string `json:"kind"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &state); err != nil || response.Code != 200 || state.Kind != "anonymous" {
		t.Fatalf("revoked role retained identity=%s,%d,%v", state.Kind, response.Code, err)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth-fixture", nil)
	signBrowserTestRequest(t, server, request, token, "")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 401 {
		t.Fatalf("revoked role retained admin API=%d", response.Code)
	}
}

func TestBrowserHTTPExpiredIdentityCookie(t *testing.T) {
	server, handler, user := testBrowserHTTP(t)
	_, token, _, err := createBrowserTestSession(t, server, user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = server.deps.Store.DB().ExecContext(context.Background(), `UPDATE sessions SET expires_at='2000-01-01T00:00:00Z' WHERE scope='browser'`); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/browser/session", nil)
	signBrowserTestRequest(t, server, request, token, "")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var state struct {
		Kind string `json:"kind"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &state); err != nil || response.Code != 200 || state.Kind != "anonymous" {
		t.Fatalf("expired identity=%s,%d,%v", state.Kind, response.Code, err)
	}
	expired := map[string]bool{}
	for _, cookie := range response.Result().Cookies() {
		if cookie.MaxAge < 0 {
			expired[cookie.Name] = true
		}
	}
	if !expired[browserSessionCookie] || !expired[browserRequestKeyCookie] {
		t.Fatal("expired cookie pair was not cleared")
	}

}

func TestStreamingBackupRequiresBrowserScopeAndSameOrigin(t *testing.T) {
	server, _, user := testBrowserHTTP(t)
	_, token, _, err := createBrowserTestSession(t, server, user)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.requireSignedRequest(server.requireSession(server.requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))))
	for _, test := range []struct {
		name   string
		origin string
		cookie bool
		status int
	}{{"valid browser", "https://panel.example", true, 204}, {"foreign origin", "https://evil.example", true, 403}, {"missing device", "https://panel.example", false, 401}} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, streamingBackupUploadPath, nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Sec-Fetch-Site", "same-origin")
			request.Header.Set("Idempotency-Key", "upload-command")
			if test.cookie {
				request.AddCookie(&http.Cookie{Name: browserSessionCookie, Value: token})
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("stream upload=%d,want=%d", response.Code, test.status)
			}
		})
	}
}

func TestBrowserOAuthRoutesAreRemoved(t *testing.T) {
	_, handler, _ := testBrowserHTTP(t)
	for _, test := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/auth/browser/telegram/start"},
		{http.MethodGet, "/api/v1/auth/browser/telegram/callback"},
		{http.MethodGet, "/api/v1/admin-access/oauth-settings"},
		{http.MethodPut, "/api/v1/admin-access/oauth-settings"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("removed route %s %s returned %d", test.method, test.path, response.Code)
		}
	}
}
