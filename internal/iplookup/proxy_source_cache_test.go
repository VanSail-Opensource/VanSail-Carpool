package iplookup

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type sourceSettings struct {
	mu    sync.Mutex
	value string
}

func enabledSourceConfig() Config {
	c := DefaultConfig()
	c.Enabled, c.LookupFeeTXB, c.RefreshFeeTXB = true, "1", "1"
	c.Providers[2].Enabled = true
	return c
}

func (v *sourceSettings) Optional(context.Context, string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.value, nil
}
func (v *sourceSettings) set(t *testing.T, c Config) {
	t.Helper()
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	v.mu.Lock()
	v.value = string(raw)
	v.mu.Unlock()
}

func awaitSource(t *testing.T, s *EnhancedService, id string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s.tests.mu.Lock()
		cache := s.tests.sources[id]
		ready := cache != nil && cache.index != nil && cache.done == nil
		s.tests.mu.Unlock()
		if ready {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("source refresh did not settle")
}

func TestSourceStartupDedupRefreshAndStaleEvidence(t *testing.T) {
	s := diagnosticService(t)
	c := enabledSourceConfig()
	c.ProxySources = []ProxySource{{ID: "one", Name: "One", URL: "https://example.net/one", Enabled: true}, {ID: "two", Name: "Two", URL: "https://example.net/two", Enabled: true}}
	settings := &sourceSettings{}
	settings.set(t, c)
	s.settings = settings
	var calls atomic.Int32
	var failed atomic.Bool
	s.inventoryFetch = func(context.Context, string, Config) ([]byte, error) {
		calls.Add(1)
		if failed.Load() {
			return nil, errors.New("download unavailable")
		}
		return []byte("8.8.8.8:443\n1.1.1.1:443\n8.8.8.8:80"), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.StartSourceTests(ctx)
	t.Cleanup(func() { cancel(); s.WaitSourceTests() })
	awaitSource(t, s, "one")
	awaitSource(t, s, "two")
	status, err := s.SourcesStatus(ctx)
	if err != nil || status.UniqueIPs != 2 || status.Sources[0].Count != 2 || calls.Load() != 2 {
		t.Fatalf("startup: %+v calls=%d err=%v", status, calls.Load(), err)
	}
	if remaining := time.Until(s.tests.sources["one"].expires); remaining < proxyInventoryTTL-time.Minute || remaining > proxyInventoryTTL {
		t.Fatal("cache TTL is not six hours")
	}
	s.tests.mu.Lock()
	s.tests.sources["one"].nextRefresh = time.Now().Add(-time.Second)
	s.tests.mu.Unlock()
	if err := s.ReconcileSources(ctx); err != nil {
		t.Fatal(err)
	}
	awaitSource(t, s, "one")
	if calls.Load() != 3 {
		t.Fatal("due refresh also refreshed another source")
	}
	failed.Store(true)
	job, err := s.TestSource(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	awaitSource(t, s, "one")
	result, err := s.SourceTest(job.ID)
	if err != nil || result.Result.Status != "error" || result.Result.Count != 2 {
		t.Fatal("refresh did not preserve cached evidence")
	}
	pool, err := s.pool(ctx, c)
	if err != nil || pool.complete || len(pool.assessment("8.8.8.8").Matches) != 2 {
		t.Fatal("failed refresh certified misses or lost positive evidence")
	}
	status, err = s.SourcesStatus(ctx)
	if err != nil || !status.Sources[0].Stale || status.Sources[1].Stale {
		t.Fatal("freshness was not isolated by source")
	}
}

func TestSourceConfigurationRejectsObsoletePublication(t *testing.T) {
	s := diagnosticService(t)
	c := enabledSourceConfig()
	c.ProxySources = []ProxySource{{ID: "one", Name: "One", URL: "https://example.net/old", Enabled: true}}
	settings := &sourceSettings{}
	settings.set(t, c)
	s.settings = settings
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	s.inventoryFetch = func(_ context.Context, url string, _ Config) ([]byte, error) {
		if url == "https://example.net/old" {
			close(entered)
			<-release
			return []byte("8.8.8.8:443"), nil
		}
		return []byte("1.1.1.1:443"), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.StartSourceTests(ctx)
	t.Cleanup(func() { cancel(); unblock(); s.WaitSourceTests() })
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("startup did not collect")
	}
	c.ProxySources[0].URL = "https://example.net/new"
	settings.set(t, c)
	if err := s.ReconcileSources(ctx); err != nil {
		t.Fatal(err)
	}
	unblock()
	awaitSource(t, s, "one")
	cancel()
	s.WaitSourceTests() // Joins obsolete jobs before inspecting their publication.
	pool, err := s.pool(context.Background(), c)
	if err != nil || len(pool.assessment("8.8.8.8").Matches) != 0 || len(pool.assessment("1.1.1.1").Matches) != 1 {
		t.Fatal("obsolete source replaced the current cache")
	}
}

func TestSourceShutdownCancelsCollection(t *testing.T) {
	s := diagnosticService(t)
	c := enabledSourceConfig()
	c.ProxySources = []ProxySource{{ID: "one", Name: "One", URL: "https://example.net/one", Enabled: true}}
	settings := &sourceSettings{}
	settings.set(t, c)
	s.settings = settings
	entered := make(chan struct{})
	s.inventoryFetch = func(ctx context.Context, _ string, _ Config) ([]byte, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.StartSourceTests(ctx)
	select {
	case <-entered:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("startup did not collect")
	}
	cancel()
	stopped := make(chan struct{})
	go func() { s.WaitSourceTests(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not join collection")
	}
	if s.tests.sources["one"].complete {
		t.Fatal("cancelled collection certified coverage")
	}
}
