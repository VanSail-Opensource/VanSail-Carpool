package httpapi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type recoveryAuthRepository struct {
	accounts.Repository
	err      error
	sessions int
}

func (r *recoveryAuthRepository) UpsertTelegramUser(context.Context, model.TelegramProfile, bool) (model.User, bool, error) {
	return model.User{}, false, r.err
}

func (r *recoveryAuthRepository) ReplaceSession(context.Context, []byte, string, time.Time) error {
	r.sessions++
	return nil
}

type recoveryAuthValidator struct{}

func (recoveryAuthValidator) Validate(string) (model.TelegramProfile, error) {
	return model.TelegramProfile{ID: 581621}, nil
}

func TestAuthenticateDistinguishesVerifiedCleanupRecovery(t *testing.T) {
	for _, test := range []struct {
		name, code string
		err        error
		status     int
	}{
		{"verified partial cleanup", "ACCOUNT_RECOVERY_PENDING", fmt.Errorf("identity transaction: %w", database.ErrAccountCleanupRecoveryPending), http.StatusConflict},
		{"other unavailable account", "INVALID_TELEGRAM_DATA", database.ErrAccountUnavailable, http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &recoveryAuthRepository{err: test.err}
			s := &Server{deps: Dependencies{
				Accounts: accounts.NewService(repo, nil, recoveryAuthValidator{}, nil, nil, nil, nil, time.Hour),
				Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
			}}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/telegram", strings.NewReader(`{"initData":"verified-fixture"}`))
			w := httptest.NewRecorder()
			s.authenticate(w, r)
			if w.Code != test.status || !strings.Contains(w.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("authentication response: %d %s", w.Code, w.Body.String())
			}
			if repo.sessions != 0 || len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), "authenticated") {
				t.Fatal("partial cleanup issued an authenticated session")
			}
			if test.status == http.StatusConflict && !strings.Contains(w.Body.String(), "Your sign-in was verified. Cleanup is paused") {
				t.Fatal("verified identity was presented as invalid authentication")
			}
		})
	}
}
