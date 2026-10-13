package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/txyyddss/Remna-User-Panel/internal/affiliates"
	"github.com/txyyddss/Remna-User-Panel/internal/browserauth"
	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/database"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/ids"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/secret"
	"github.com/txyyddss/Remna-User-Panel/internal/requestauth"
)

func testBrowserHTTP(t *testing.T) (*Server, http.Handler, model.User) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "browser-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	store := database.NewStore(db)
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	public, err := url.Parse("https://panel.example")
	if err != nil {
		t.Fatal(err)
	}
	user, _, err := store.UpsertTelegramUser(context.Background(), model.TelegramProfile{ID: 80305, FirstName: "Admin"}, true)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := requestauth.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{deps: Dependencies{Store: store, PublicURL: public, SecureCookies: true, BrowserAuth: browserauth.New(store, vault, nil, public, []int64{user.TelegramID}), Affiliates: affiliates.NewService(nil, nil)}, requests: verifier, authLimiter: newAuthLimiter(), adminTelegramIDs: map[int64]struct{}{user.TelegramID: {}}}
	router := chi.NewRouter()
	server.mountBrowserAuth(router)
	return server, router, user
}

func signBrowserTestRequest(t *testing.T, server *Server, r *http.Request, token, body string) {
	t.Helper()
	key, err := server.requests.ClientKey(token)
	if err != nil {
		t.Fatal(err)
	}
	r.AddCookie(&http.Cookie{Name: browserSessionCookie, Value: token})
	r.AddCookie(&http.Cookie{Name: browserRequestKeyCookie, Value: key})
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := ids.Token(16)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := requestauth.Sign(key, r.Method, requestauth.CanonicalTarget(r.URL), timestamp, nonce, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set(requestauth.TimestampHeader, timestamp)
	r.Header.Set(requestauth.NonceHeader, nonce)
	r.Header.Set(requestauth.SignatureHeader, signature)
}

func TestBrowserHTTPExchangeOriginsCookiesRenewAndLogout(t *testing.T) {
	server, handler, user := testBrowserHTTP(t)
	link, _, err := server.deps.BrowserAuth.Link(context.Background(), user, "/admin/users")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"token":"` + strings.TrimPrefix(parsed.Fragment, "token=") + `"}`
	for _, origin := range []string{"", "https://evil.example"} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/browser/exchange", strings.NewReader(raw))
		request.Header.Set("Origin", origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 403 {
			t.Fatalf("cross-origin exchange=%d", response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/browser/exchange", strings.NewReader(raw))
	request.Header.Set("Origin", "https://panel.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("exchange=%d:%s", response.Code, response.Body.String())
	}
	var token, key string
	for _, cookie := range response.Result().Cookies() {
		switch cookie.Name {
		case browserSessionCookie:
			token = cookie.Value
			if !cookie.HttpOnly || !cookie.Secure || cookie.MaxAge < 29*24*3600 {
				t.Fatal("weak session cookie")
			}
		case browserRequestKeyCookie:
			key = cookie.Value
			if cookie.HttpOnly || !cookie.Secure {
				t.Fatal("unusable signing cookie")
			}
		}
	}
	if token == "" || key == "" {
		t.Fatal("browser cookie pair missing")
	}
	for _, path := range []string{"/api/v1/auth/browser/session", "/api/v1/auth/browser/session/renew"} {
		method := http.MethodGet
		if strings.HasSuffix(path, "/renew") {
			method = http.MethodPost
		}
		request := httptest.NewRequest(method, path, nil)
		request.Header.Set("Origin", "https://panel.example")
		signBrowserTestRequest(t, server, request, token, "")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("session path=%s,status=%d:%s", path, response.Code, response.Body.String())
		}
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name == browserSessionCookie && cookie.Value != token {
				t.Fatal("renewal rotated token")
			}
			if cookie.Name == browserRequestKeyCookie && cookie.Value != key {
				t.Fatal("renewal rotated key")
			}
		}
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/browser/logout", nil)
	request.Header.Set("Origin", "https://panel.example")
	signBrowserTestRequest(t, server, request, token, "")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("logout=%d:%s", response.Code, response.Body.String())
	}
	if _, _, err = server.deps.BrowserAuth.Session(context.Background(), token); err == nil {
		t.Fatal("logout left a live device")
	}
}

func TestBrowserHTTPAnonymousUnsignedAndMemberResult(t *testing.T) {
	server, handler, _ := testBrowserHTTP(t)
	for _, test := range []struct {
		name   string
		member bool
		kind   string
	}{{"anonymous", false, "anonymous"}, {"member", true, "member"}} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/browser/session", nil)
			if test.member {
				state := browserMemberState{Profile: model.TelegramProfile{ID: 999, FirstName: "Member"}, Expires: time.Now().Add(time.Minute)}
				encoded, err := server.deps.BrowserAuth.SealCookie(browserMemberCookie, state)
				if err != nil {
					t.Fatal(err)
				}
				request.AddCookie(&http.Cookie{Name: browserMemberCookie, Value: encoded})
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			var state struct {
				Authenticated bool   `json:"authenticated"`
				Kind          string `json:"kind"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil || response.Code != 200 || state.Authenticated || state.Kind != test.kind {
				t.Fatalf("identity=%+v,status=%d,error=%v", state, response.Code, err)
			}
		})
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/browser/telegram/callback?state=missing&error=access_denied", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 303 || response.Header().Get("Location") != "/admin/login?error=OAUTH_STATE_INVALID" {
		t.Fatalf("unbound callback=%d,%s", response.Code, response.Header().Get("Location"))
	}
}
