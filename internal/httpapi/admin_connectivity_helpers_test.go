package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type connectivityHTTPSettings string

func (value connectivityHTTPSettings) Optional(context.Context, string) (string, error) {
	return string(value), nil
}

type connectivityHTTPSource struct{ target connectivity.Target }

func (source connectivityHTTPSource) Resolve(_ context.Context, username string) (connectivity.User, error) {
	if username != "monitor" {
		return connectivity.User{}, &connectivity.CodeError{Code: "CONNECTIVITY_INVALID_USERNAME"}
	}
	return connectivity.User{ID: 42, Username: "monitor", Status: "ACTIVE"}, nil
}

func (connectivityHTTPSource) Validate(context.Context, int64) error { return nil }

func (source connectivityHTTPSource) Load(context.Context, int64) (connectivity.User, []connectivity.Target, error) {
	return connectivity.User{ID: 42, Username: "monitor", Status: "ACTIVE"}, []connectivity.Target{source.target}, nil
}

type connectivityHTTPProbe struct {
	started chan struct{}
	release chan struct{}
}

func (probe *connectivityHTTPProbe) Check(ctx context.Context, _ connectivity.Target, _ connectivity.Config) connectivity.Outcome {
	select {
	case probe.started <- struct{}{}:
	default:
	}
	select {
	case <-probe.release:
		return connectivity.Outcome{Status: "connected"}
	case <-ctx.Done():
		return connectivity.Outcome{Status: "interrupted", ErrorCode: "CONNECTIVITY_INTERRUPTED"}
	}
}

func connectivityHTTPFixture(t *testing.T) (http.Handler, *database.Store, *connectivityHTTPProbe, string) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "connectivity.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close fixture database: %v", err)
		}
	})
	store := database.NewStore(db)
	cfg := connectivity.DefaultConfig()
	cfg.ProxyUserID = 42
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "connectivity", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := queue.Start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	probe := &connectivityHTTPProbe{started: make(chan struct{}, 1), release: make(chan struct{})}
	host := uuid.NewString()
	source := connectivityHTTPSource{target: connectivity.Target{HostUUID: host, Remark: "host", Address: "example.com", Port: 443,
		Resolved: json.RawMessage(`{"protocolOptions":{"password":"private-fixture-secret"}}`)}}
	service := connectivity.NewService(connectivityHTTPSettings(encoded), source, probe, store, queue)
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("stop fixture checker: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("checker did not stop")
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := queue.Shutdown(shutdownCtx); err != nil {
			t.Errorf("stop fixture queue: %v", err)
		}
	})
	connectivityHTTPWait(t, func() bool {
		snapshot, err := service.Snapshot(context.Background())
		return err == nil && !snapshot.Stale
	})
	server := &Server{deps: Dependencies{Connectivity: service, Store: store}, adminTelegramIDs: map[int64]struct{}{99: {}}}
	router := chi.NewRouter()
	router.Route("/api/v1/admin", func(admin chi.Router) {
		admin.Use(server.requireAdmin)
		server.mountAdmin(admin)
	})
	return router, store, probe, host
}

func connectivityHTTPRequest(handler http.Handler, method, path, body string, user model.User) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request = request.WithContext(context.WithValue(request.Context(), userContextKey, user))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func connectivityHTTPWait(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ready() {
		if !time.Now().Before(deadline) {
			t.Fatal("HTTP fixture condition timed out")
		}
		time.Sleep(time.Millisecond)
	}
}
