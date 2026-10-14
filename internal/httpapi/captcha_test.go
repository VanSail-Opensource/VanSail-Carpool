package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/turnstile"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type entryTestRepository struct {
	qualified bool
	writes    int
}

func (r *entryTestRepository) RegisterPanelEntry(context.Context, int64, bool) error { return nil }
func (r *entryTestRepository) PanelEntryQualified(context.Context, int64) (bool, error) {
	return r.qualified, nil
}
func (r *entryTestRepository) VerifyPanelEntry(context.Context, int64) error {
	r.qualified = true
	r.writes++
	return nil
}

type entryTestSettings struct{}

func (entryTestSettings) Optional(_ context.Context, key string) (string, error) {
	switch key {
	case "captcha.turnstile.enabled":
		return "true", nil
	case "captcha.turnstile.site_key":
		return "site-key", nil
	default:
		return "secret", nil
	}
}

type entryTestVerifier struct{ err error }

func (v entryTestVerifier) Verify(context.Context, string, string, string) error { return v.err }

func captchaTestServer(repo *entryTestRepository, err error) *Server {
	return &Server{deps: Dependencies{FirstEntry: &accounts.FirstEntry{Repository: repo, Settings: entryTestSettings{}, Verifier: entryTestVerifier{err: err}, Hostname: "panel.example", AdminTelegramIDs: []int64{42}}}}
}

func TestCaptchaGateRejectsProtectedReadsAndOnboardingMutations(t *testing.T) {
	t.Parallel()
	server := captchaTestServer(&entryTestRepository{}, nil)
	user := model.User{ID: "member", TelegramID: 42, Role: "user"}
	for _, path := range []string{"/api/v1/catalog", "/api/v1/dashboard", "/api/v1/onboarding/username", "/api/v1/onboarding/agreement", "/api/v1/onboarding/content", "/api/v1/payments/orders", "/api/v1/community/invites/group"} {
		r := communityRequest("POST", path, user)
		response := httptest.NewRecorder()
		server.requireCaptcha(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("protected handler reached before verification") })).ServeHTTP(response, r)
		if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "CAPTCHA_REQUIRED") {
			t.Fatalf("%s=%d %s", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"/api/v1/me", "/api/v1/me/captcha", "/api/v1/me/display-currency"} {
		response := httptest.NewRecorder()
		server.requireCaptcha(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(response, communityRequest("GET", path, user))
		if response.Code != http.StatusNoContent {
			t.Fatalf("bootstrap %s=%d", path, response.Code)
		}
	}
	user.Role = "admin"
	response := httptest.NewRecorder()
	server.requireCaptcha(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(response, communityRequest("GET", "/api/v1/admin/settings", user))
	if response.Code != http.StatusNoContent {
		t.Fatal("admin configuration gated")
	}
}

func TestCaptchaProviderFailureCannotSetVerificationFlag(t *testing.T) {
	t.Parallel()
	for _, failure := range []error{turnstile.ErrRejected, turnstile.ErrUnavailable} {
		repo := &entryTestRepository{}
		server := captchaTestServer(repo, failure)
		r := communityRequest("POST", "/api/v1/me/captcha", model.User{ID: "member", TelegramID: 42, Role: "user"})
		r.Body = io.NopCloser(strings.NewReader(`{"token":"replayed-token"}`))
		response := httptest.NewRecorder()
		server.verifyCaptcha(response, r)
		if response.Code < 400 || repo.writes != 0 || repo.qualified {
			t.Fatalf("failed challenge qualified=%v writes=%d status=%d", repo.qualified, repo.writes, response.Code)
		}
	}
}
