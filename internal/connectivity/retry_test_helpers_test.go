package connectivity

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func retryFixture(t *testing.T, cfg Config, probe Probe) (*Service, *testRepository, *work) {
	t.Helper()
	settings, source, repository, _ := configuredDependencies(t)
	settings.set(t, cfg)
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "connectivity", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := queue.Start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := queue.Shutdown(shutdownCtx); err != nil {
			t.Errorf("stop retry queue: %v", err)
		}
	})
	service := NewService(settings, source, probe, repository, queue)
	service.hash = ConfigHash(cfg)
	item := &work{config: cfg, hash: service.hash, ctx: ctx, cancel: cancel,
		run: Run{ID: "retry-run", Trigger: "manual", Status: "running", StartedAt: time.Now().UTC()}}
	service.active = item
	return service, repository, item
}
