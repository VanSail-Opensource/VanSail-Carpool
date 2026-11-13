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
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type onboardingStorageRepository struct {
	accounts.Repository
	boundary string
	err      error
}

func (r onboardingStorageRepository) BeginAgreement(context.Context, string) error { return r.err }
func (r onboardingStorageRepository) CurrentAgreementContract(context.Context) (int, []string, error) {
	if r.boundary == "contract" {
		return 0, nil, r.err
	}
	return 1, []string{"terms"}, nil
}
func (r onboardingStorageRepository) CompleteOnboardingRevision(context.Context, string, int, []string, time.Time) (model.User, error) {
	return model.User{}, r.err
}

func TestOnboardingStorageOutagesRemainLocalRetryableFailures(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"continue", "contract", "completion"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			repository := onboardingStorageRepository{boundary: boundary, err: errors.New("database connection refused")}
			server := &Server{deps: Dependencies{
				Accounts: accounts.NewService(repository, nil, nil, nil, nil, nil, nil, time.Hour),
				Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
			}}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/onboarding/agreement", strings.NewReader(`{"revision":1,"agreementIds":["terms"]}`))
			request = request.WithContext(context.WithValue(request.Context(), userContextKey, model.User{ID: "member", OnboardingState: "agreement"}))
			response := httptest.NewRecorder()
			if boundary == "continue" {
				server.continueOnboarding(response, request)
			} else {
				server.acceptAgreement(response, request)
			}
			if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"ONBOARDING_UNAVAILABLE"`) || strings.Contains(response.Body.String(), "XNODE") {
				t.Fatalf("local persistence outage was classified as a conflict or native provisioning failure: %d %s", response.Code, response.Body.String())
			}
		})
	}
}
