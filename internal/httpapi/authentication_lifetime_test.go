package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/requestauth"
)

type lifetimeAuthRepository struct {
	accounts.Repository
	user      model.User
	hash      []byte
	expiresAt time.Time
	now       time.Time
}

func (r *lifetimeAuthRepository) UpsertTelegramUser(context.Context, model.TelegramProfile, bool) (model.User, bool, error) {
	return r.user, false, nil
}

func (r *lifetimeAuthRepository) ReplaceSession(_ context.Context, hash []byte, userID string, expiresAt time.Time) error {
	if userID != r.user.ID {
		return errors.New("unexpected session owner")
	}
	r.hash, r.expiresAt = bytes.Clone(hash), expiresAt
	return nil
}

func (r *lifetimeAuthRepository) UserBySession(_ context.Context, hash []byte, at time.Time) (model.User, error) {
	// Advance the persistence clock without sleeping or weakening HTTP signing.
	if r.now.After(at) {
		at = r.now
	}
	if !bytes.Equal(hash, r.hash) || !r.expiresAt.After(at) {
		return model.User{}, errors.New("session expired or unknown")
	}
	return r.user, nil
}

func TestConfiguredAuthenticationSurvivesBrowserCookieRoundTrip(t *testing.T) {
	for name, value := range map[string]string{
		"ADMIN_TELEGRAM_ID": "42", "TELEGRAM_BOT_TOKEN": "42:fixture-token",
		"PUBLIC_BASE_URL": "https://panel.example", "TZ": "UTC",
		"CONFIG_MASTER_KEY": base64.StdEncoding.EncodeToString(make([]byte, 32)),
		"DB_DRIVER":         "postgres", "DATABASE_URL": "postgres://fixture:secret@db/vansail",
		"DATABASE_URL_FILE": "", "ALLOW_INSECURE_HTTP": "", "TRUSTED_PROXY_CIDRS": "",
	} {
		t.Setenv(name, value)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := requestauth.New(cfg.MasterKey)
	if err != nil {
		t.Fatal(err)
	}
	repo := &lifetimeAuthRepository{user: model.User{
		ID: "session-member", TelegramID: 581621, Role: "user", OnboardingState: "intro",
	}}
	server := &Server{requests: verifier, deps: Dependencies{
		Accounts:   accounts.NewService(repo, nil, recoveryAuthValidator{}, nil, nil, nil, cfg.AdminTelegramIDs, cfg.SessionTTL),
		SessionTTL: cfg.SessionTTL, SecureCookies: cfg.PublicBaseURL.Scheme == "https",
	}}
	startedAt := time.Now().UTC()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/telegram", strings.NewReader(`{"initData":"verified-fixture"}`))
	response := httptest.NewRecorder()
	server.authenticate(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("authentication status = %d", response.Code)
	}
	if !repo.expiresAt.After(startedAt.Add(6*24*time.Hour)) || repo.expiresAt.After(time.Now().Add(7*24*time.Hour)) {
		t.Fatalf("stored session expires at %s", repo.expiresAt)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("authentication cookie count = %d", len(cookies))
	}
	for _, cookie := range cookies {
		if cookie.MaxAge != 7*24*3600 || !cookie.Expires.After(startedAt.Add(6*24*time.Hour)) || !cookie.Secure || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode {
			t.Fatalf("invalid lifetime or security flags for %s", cookie.Name)
		}
		if cookie.HttpOnly != (cookie.Name == sessionCookie) {
			t.Fatalf("invalid cookie visibility for %s", cookie.Name)
		}
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(cfg.PublicBaseURL, cookies)
	retained := jar.Cookies(cfg.PublicBaseURL)
	if len(retained) != 2 {
		t.Fatalf("browser retained %d authentication cookies", len(retained))
	}
	var clientKey string
	for _, cookie := range retained {
		if cookie.Name == requestauth.ClientKeyCookie {
			clientKey = cookie.Value
		}
	}
	repo.now = startedAt.Add(time.Minute)
	request = httptest.NewRequest(http.MethodGet, cfg.PublicBaseURL.String()+"/api/v1/me", nil)
	for _, cookie := range retained {
		request.AddCookie(cookie)
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "AAAAAAAAAAAAAAAAAAAAAA"
	signature, err := requestauth.Sign(clientKey, request.Method, requestauth.CanonicalTarget(request.URL), timestamp, nonce, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set(requestauth.TimestampHeader, timestamp)
	request.Header.Set(requestauth.NonceHeader, nonce)
	request.Header.Set(requestauth.SignatureHeader, signature)
	response = httptest.NewRecorder()
	server.requireSignedRequest(server.requireSession(http.HandlerFunc(server.me))).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"id":"session-member"`) {
		t.Fatalf("signed session lookup after one minute = %d: %s", response.Code, response.Body.String())
	}
}
