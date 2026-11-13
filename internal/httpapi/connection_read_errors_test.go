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
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
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

func (*connectionReadRepository) UserByID(_ context.Context, id string) (model.User, error) {
	return model.User{ID: id, Role: "admin", AccountStatus: "active"}, nil
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
			request = request.WithContext(context.WithValue(request.Context(), userContextKey, model.User{ID: "administrator"}))
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

func TestAdministratorScanHandlersCarryOriginalActorAfterDemotion(t *testing.T) {
	server, _, actor := testBrowserHTTP(t)
	ctx := context.Background()
	owner, _, err := server.deps.Store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: actor.TelegramID + 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	server.deps.Connections = connections.NewService(server.deps.Store, nil, nil)
	if _, err := server.deps.Store.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	for _, handler := range []http.HandlerFunc{server.adminRequestUserConnections, server.adminPollUserConnections} {
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		request.Header.Set("Idempotency-Key", "stale-administrator")
		route := chi.NewRouteContext()
		route.URLParams.Add("userId", owner.ID)
		route.URLParams.Add("scanId", "private-scan")
		request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, route))
		request = request.WithContext(context.WithValue(request.Context(), userContextKey, actor))
		response := httptest.NewRecorder()
		handler(response, request)
		if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "ADMIN_REQUIRED") {
			t.Fatalf("stale HTTP actor reached diagnostics: %d %s", response.Code, response.Body.String())
		}
	}
	var count int
	if err := server.deps.Store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM connection_scans`).Scan(&count); err != nil || count != 0 {
		t.Fatal("rejected HTTP actor queued a scan", count, err)
	}
}
