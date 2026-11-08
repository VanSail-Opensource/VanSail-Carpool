package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/go-chi/chi/v5"
)

var hourlyFinancialCommands = []string{"hourly.credit.install", "hourly.credit.seal", "hourly.credit.ack"}

func TestPublicNativeCommandRejectsHourlyCreditsBeforePeerLookup(t *testing.T) {
	master := &xnode.Master{}
	for _, name := range hourlyFinancialCommands {
		t.Run(name, func(t *testing.T) {
			raw, err := master.Command(context.Background(), "missing", "command", name, nil)
			if !errors.Is(err, xnode.ErrInvalid) || raw != nil {
				t.Fatalf("financial command reached peer lookup: error=%v", err)
			}
		})
	}
	if _, err := master.Command(context.Background(), "missing", "ordinary", "runtime.reload", nil); !errors.Is(err, xnode.ErrUnavailable) {
		t.Fatalf("ordinary command no longer reaches peer lookup: %v", err)
	}
}

type hourlyCommandBoundaryRepository struct {
	*database.Store
	replays, creates, nodeReads int
}

func (r *hourlyCommandBoundaryRepository) ReplayXNodeCommand(context.Context, string, string, string) (model.OperationReceipt, bool, error) {
	r.replays++
	return model.OperationReceipt{ID: "existing", Kind: xnode.OperationKind, Status: "queued"}, true, nil
}

func (r *hourlyCommandBoundaryRepository) CreateProviderOperation(context.Context, providerops.CreateInput, time.Time) (providerops.Operation, bool, error) {
	r.creates++
	return providerops.Operation{}, false, xnode.ErrUnavailable
}

func (r *hourlyCommandBoundaryRepository) XNodeNode(context.Context, string) (xnode.Node, error) {
	r.nodeReads++
	return xnode.Node{}, xnode.ErrNotFound
}

func (r *hourlyCommandBoundaryRepository) assertUntouched(t *testing.T) {
	t.Helper()
	if r.replays != 0 || r.creates != 0 || r.nodeReads != 0 {
		t.Fatalf("rejected command accessed state: replays=%d creates=%d nodes=%d", r.replays, r.creates, r.nodeReads)
	}
}

func TestQueuedHourlyCreditsRejectBeforeReplayOrPersistence(t *testing.T) {
	repo := &hourlyCommandBoundaryRepository{}
	service := xnode.NewService(repo, nil, nil, nil, nil)
	for _, name := range hourlyFinancialCommands {
		t.Run(name, func(t *testing.T) {
			receipt, err := service.QueueCommand(context.Background(), "admin", "node", "existing-key", xnode.CommandRequest{Name: name})
			if !errors.Is(err, xnode.ErrInvalid) || receipt.ID != "" {
				t.Fatalf("financial command accepted: error=%v", err)
			}
			repo.assertUntouched(t)
		})
	}
	// A normal command with the same key can reach replay, proving admission
	// rejected the financial names rather than a broken repository fixture.
	receipt, err := service.QueueCommand(context.Background(), "admin", "node", "existing-key", xnode.CommandRequest{Name: "update.check"})
	if err != nil || receipt.ID != "existing" || repo.replays != 1 || repo.creates != 0 || repo.nodeReads != 0 {
		t.Fatalf("ordinary command replay failed: error=%v", err)
	}
}

func TestXNodeWriteHTTPRejectsHourlyCreditCommands(t *testing.T) {
	s, handler, user := testBrowserHTTP(t)
	repo := &hourlyCommandBoundaryRepository{}
	s.deps.XNode = xnode.NewService(repo, nil, nil, nil, nil)
	s.deps.APITokens = apitoken.New(s.deps.Store, []int64{user.TelegramID})
	router := handler.(chi.Router)
	router.With(s.requireSignedRequest, s.requireSession, s.requireAdmin).Post("/api/v1/admin/xnode/nodes/{id}/commands", s.xnodeQueueCommand)
	issued, err := s.deps.APITokens.Create(context.Background(), user, apitoken.CreateInput{Name: "node automation", Scopes: []string{"xnode:write"}})
	if err != nil {
		t.Fatal(err)
	}
	request := func(name string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/xnode/nodes/node/commands", strings.NewReader(`{"name":"`+name+`"}`))
		r.Header.Set("Authorization", "Bearer "+issued.Token)
		r.Header.Set("Idempotency-Key", "existing-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	for _, name := range hourlyFinancialCommands {
		t.Run(name, func(t *testing.T) {
			response := request(name)
			if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "XNODE_INPUT_INVALID") {
				t.Fatalf("financial command HTTP status=%d", response.Code)
			}
			repo.assertUntouched(t)
		})
	}
	if response := request("update.check"); response.Code != http.StatusAccepted || repo.replays != 1 {
		t.Fatalf("authorized ordinary command HTTP status=%d", response.Code)
	}
}
