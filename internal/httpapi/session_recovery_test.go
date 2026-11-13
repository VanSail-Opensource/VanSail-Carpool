package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type sessionRecoveryRepository struct {
	accounts.Repository
	err   error
	calls int
}

func (r *sessionRecoveryRepository) UserBySession(context.Context, []byte, time.Time) (model.User, error) {
	r.calls++
	return model.User{ID: "retained-session-owner"}, r.err
}

func TestMiniAppSessionStorageOutageIsRetryableWithoutLosingIdentity(t *testing.T) {
	t.Parallel()
	repository := &sessionRecoveryRepository{}
	server := &Server{deps: Dependencies{Accounts: accounts.NewService(repository, nil, nil, nil, nil, nil, nil, time.Hour)}}
	protectedCalls := 0
	handler := server.requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		protectedCalls++
		if currentUser(r).ID != "retained-session-owner" {
			t.Fatal("recovered request lost its authenticated owner")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, test := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"database outage", errors.New("private storage diagnostic"), http.StatusServiceUnavailable, "SESSION_UNAVAILABLE"},
		{"request timeout", context.DeadlineExceeded, http.StatusServiceUnavailable, "SESSION_UNAVAILABLE"},
		{"same cookie after recovery", nil, http.StatusNoContent, ""},
		{"expired session", database.ErrNotFound, http.StatusUnauthorized, "UNAUTHENTICATED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository.err = test.err
			request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			request.AddCookie(&http.Cookie{Name: sessionCookie, Value: "retained-bearer"})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || test.code != "" && !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("session lookup = %d: %s", response.Code, response.Body.String())
			}
			if len(response.Result().Cookies()) != 0 || strings.Contains(response.Body.String(), "private storage") {
				t.Fatal("lookup failure replaced credentials or exposed a storage diagnostic")
			}
		})
	}
	if repository.calls != 4 || protectedCalls != 1 {
		t.Fatalf("lookups = %d; protected requests = %d", repository.calls, protectedCalls)
	}
}
