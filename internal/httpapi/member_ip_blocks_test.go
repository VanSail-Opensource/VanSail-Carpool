package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/go-chi/chi/v5"
)

type ipBlockHTTPRepository struct {
	connections.BlockRepository
	ownerID  string
	actorID  string
	conflict bool
}

func (r *ipBlockHTTPRepository) ProviderOperationForActorKey(context.Context, string, string, string, string) (model.OperationReceipt, bool, error) {
	return model.OperationReceipt{}, false, nil
}

func (r *ipBlockHTTPRepository) ConnectionIPBlockForUser(_ context.Context, blockID, ownerID string) (connections.IPBlockRecord, error) {
	if blockID != "block-1" || ownerID != r.ownerID {
		return connections.IPBlockRecord{}, connections.ErrIPBlockNotFound
	}
	return connections.IPBlockRecord{ID: blockID, UserID: ownerID, IPDigest: "digest"}, nil
}

func (r *ipBlockHTTPRepository) BeginConnectionIPUnblock(_ context.Context, _, ownerID string,
	input providerops.CreateInput, _ time.Time) (providerops.Operation, bool, error) {
	if r.conflict {
		return providerops.Operation{}, false, database.ErrConflict
	}
	r.actorID = input.ActorUserID
	return providerops.Operation{Receipt: model.OperationReceipt{ID: "operation-1", Kind: connections.UnblockOperationKind,
		Status: string(providerops.StatusQueued)}, OwnerUserID: ownerID}, false, nil
}

func TestMemberIPUnblockHidesOwnershipMismatch(t *testing.T) {
	t.Parallel()
	repository := &ipBlockHTTPRepository{ownerID: "owner"}
	server := &Server{deps: Dependencies{ConnectionDrops: connections.NewDropService(repository, nil, nil)}}
	response := httptest.NewRecorder()
	server.memberUnblockIP(response, ipBlockRequest("other", ""))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestAdminIPUnblockUsesAdministratorAsActor(t *testing.T) {
	t.Parallel()
	repository := &ipBlockHTTPRepository{ownerID: "owner"}
	server := &Server{deps: Dependencies{ConnectionDrops: connections.NewDropService(repository, nil, nil)}}
	response := httptest.NewRecorder()
	server.adminUnblockIP(response, ipBlockRequest("admin", "owner"))
	if response.Code != http.StatusAccepted || repository.actorID != "admin" {
		t.Fatalf("status = %d actor = %q", response.Code, repository.actorID)
	}
}

func TestMemberIPUnblockMapsOpenOperationToConflict(t *testing.T) {
	t.Parallel()
	repository := &ipBlockHTTPRepository{ownerID: "owner", conflict: true}
	server := &Server{deps: Dependencies{ConnectionDrops: connections.NewDropService(repository, nil, nil)}}
	response := httptest.NewRecorder()
	server.memberUnblockIP(response, ipBlockRequest("owner", ""))
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func ipBlockRequest(actorID, ownerID string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Header.Set("Idempotency-Key", "request-key")
	route := chi.NewRouteContext()
	route.URLParams.Add("blockId", "block-1")
	if ownerID != "" {
		route.URLParams.Add("userId", ownerID)
	}
	ctx := context.WithValue(request.Context(), chi.RouteCtxKey, route)
	ctx = context.WithValue(ctx, userContextKey, model.User{ID: actorID, OnboardingState: "complete"})
	return request.WithContext(ctx)
}
