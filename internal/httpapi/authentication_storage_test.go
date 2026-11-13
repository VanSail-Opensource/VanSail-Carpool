package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
)

func TestVerifiedTelegramSignInStorageFailuresRemainRetryable(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"account", "session"} {
		for _, failure := range []error{errors.New("storage temporarily unavailable"), context.DeadlineExceeded} {
			t.Run(boundary+"/"+failure.Error(), func(t *testing.T) {
				t.Parallel()
				repo := &recoveryAuthRepository{}
				wantSessionWrites := 0
				if boundary == "account" {
					repo.err = failure
				} else {
					repo.sessionErr, wantSessionWrites = failure, 1
				}
				server := &Server{deps: Dependencies{
					Accounts: accounts.NewService(repo, nil, recoveryAuthValidator{}, nil, nil, nil, nil, time.Hour),
					Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
				}}
				request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/telegram", strings.NewReader(`{"initData":"verified-fixture"}`))
				response := httptest.NewRecorder()
				server.authenticate(response, request)
				if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"AUTH_UNAVAILABLE"`) || response.Header().Get("Retry-After") == "" {
					t.Fatalf("verified sign-in was classified as invalid credentials: %d %s", response.Code, response.Body.String())
				}
				if repo.sessions != wantSessionWrites || len(response.Result().Cookies()) != 0 || strings.Contains(response.Body.String(), failure.Error()) {
					t.Fatal("failed sign-in replaced cookies or exposed internal failure", response.Header(), response.Body.String())
				}
			})
		}
	}
}
