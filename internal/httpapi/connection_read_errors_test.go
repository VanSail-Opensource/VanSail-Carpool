package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/go-chi/chi/v5"
)

type connectionReadRepository struct {
	connections.Repository
	err   error
	calls int
}

func (r *connectionReadRepository) ConnectionScanForUser(_ context.Context, scanID, userID string) (providerops.ConnectionScan, error) {
	r.calls++
	if scanID != "scan" || userID != "owner" {
		return providerops.ConnectionScan{}, errors.New("unexpected scan scope")
	}
	return providerops.ConnectionScan{}, r.err
}

func TestConnectionPollHandlerPreservesRetryableStorageFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"missing", connections.ErrScanNotFound, http.StatusNotFound, "CONNECTION_SCAN_NOT_FOUND"},
		{"wrapped missing", errors.Join(database.ErrNotFound, connections.ErrScanNotFound), http.StatusNotFound, "CONNECTION_SCAN_NOT_FOUND"},
		{"storage failure", errors.New("private database failure"), http.StatusServiceUnavailable, "CONNECTION_SCAN_UNAVAILABLE"},
		{"deadline", context.DeadlineExceeded, http.StatusServiceUnavailable, "CONNECTION_SCAN_UNAVAILABLE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &connectionReadRepository{err: test.err}
			server := &Server{deps: Dependencies{Connections: connections.NewService(repository, nil, nil)}}
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			routeContext := chi.NewRouteContext()
			routeContext.URLParams.Add("userId", "owner")
			routeContext.URLParams.Add("scanId", "scan")
			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
			response := httptest.NewRecorder()
			server.adminPollUserConnections(response, request)
			var failure apiError
			if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
				t.Fatal(err)
			}
			if response.Code != test.status || failure.Code != test.code || repository.calls != 1 {
				t.Fatalf("status=%d code=%s reads=%d; want %d/%s/1", response.Code, failure.Code, repository.calls, test.status, test.code)
			}
			if strings.Contains(response.Body.String(), "private database failure") {
				t.Fatal("response exposed storage diagnostics")
			}
		})
	}
}
