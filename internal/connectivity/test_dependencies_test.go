package connectivity

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/google/uuid"
)

type testSettings struct {
	mu    sync.RWMutex
	value string
}

func (s *testSettings) Optional(context.Context, string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value, nil
}

func (s *testSettings) set(t *testing.T, cfg Config) {
	t.Helper()
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.value = string(encoded)
	s.mu.Unlock()
}

type testSource struct {
	mu      sync.Mutex
	user    User
	targets []Target
	err     error
	loads   int
}

func (s *testSource) Resolve(context.Context, string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.user, s.err
}

func (s *testSource) Validate(context.Context, int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *testSource) Load(context.Context, int64) (User, []Target, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loads++
	return s.user, cloneTargets(s.targets), s.err
}

func (s *testSource) fail(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}

type testProbe func(context.Context, Target, Config) Outcome

func (p testProbe) Check(ctx context.Context, target Target, cfg Config) Outcome {
	return p(ctx, target, cfg)
}

func configuredDependencies(t *testing.T) (*testSettings, *testSource, *testRepository, Config) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.RemnawaveUserID = 42
	settings := &testSettings{}
	settings.set(t, cfg)
	source := &testSource{user: User{ID: 42, Username: "monitor", Status: "ACTIVE"},
		targets: []Target{{HostUUID: uuid.NewString(), Remark: "test", Address: "example.com", Port: 443,
			Resolved: json.RawMessage(`{"protocolOptions":{"password":"private-secret"}}`)}}}
	return settings, source, &testRepository{}, cfg
}

func startTestService(t *testing.T, settings Settings, source Source, probe Probe, repository Repository) *Service {
	return startTestServiceClock(t, settings, source, probe, repository, time.Now)
}

func startTestServiceClock(t *testing.T, settings Settings, source Source, probe Probe, repository Repository, now func() time.Time) *Service {
	t.Helper()
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "connectivity", Capacity: 4})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := queue.Start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	service := NewService(settings, source, probe, repository, queue)
	service.now = now
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("runtime stopped with error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("runtime did not join its worker")
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := queue.Shutdown(shutdownCtx); err != nil {
			t.Errorf("network queue did not stop: %v", err)
		}
	})
	waitFor(t, func() bool {
		service.mu.RLock()
		defer service.mu.RUnlock()
		return service.ready
	})
	return service
}

func waitFor(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ready() {
		if !time.Now().Before(deadline) {
			t.Fatal("condition did not complete before timeout")
		}
		time.Sleep(time.Millisecond)
	}
}

func waitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("signal did not arrive before timeout")
	}
}
