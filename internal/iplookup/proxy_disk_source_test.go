package iplookup

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDiskSharedSourceRefreshAndExpiryKeepSnapshotOwnership(t *testing.T) {
	s := diskService(t, 32<<20)
	c := enabledSourceConfig()
	c.ProxySources = []ProxySource{{ID: "one", Name: "One", URL: "https://example.net/one", Enabled: true}}
	settings := &sourceSettings{}
	settings.set(t, c)
	s.settings = settings
	var fail atomic.Bool
	s.inventoryFetch = func(context.Context, string, Config) ([]byte, error) {
		if fail.Load() {
			return nil, errors.New("download failed")
		}
		return []byte("11.0.0.1\n2001:ac8::/48"), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.StartSourceTests(ctx)
	t.Cleanup(func() { cancel(); s.WaitSourceTests() })
	awaitSource(t, s, "one")
	old, err := s.pool(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer old.release()
	s.tests.mu.Lock()
	expiry := s.tests.sources["one"].expires
	s.tests.mu.Unlock()
	fail.Store(true)
	job, err := s.TestSource(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	waitDiskSource(t, s, job.ID)
	s.tests.mu.Lock()
	current := s.tests.sources["one"]
	if current.expires != expiry || current.index.count() != 2 || current.complete {
		t.Error("failed refresh changed freshness or lost positives")
	}
	current.expires = time.Now().Add(-time.Second)
	s.tests.mu.Unlock()
	job, err = s.TestSource(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	waitDiskSource(t, s, job.ID)
	if err := s.disk.prune(ctx); err != nil {
		t.Fatal(err)
	}
	assessment, err := old.assessmentContext(ctx, "11.0.0.1")
	if err != nil || len(assessment.Matches) != 1 {
		t.Fatal("retired snapshot disappeared", err)
	}
	fresh, err := s.pool(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.release()
	assessment, err = fresh.assessmentContext(ctx, "11.0.0.1")
	if err != nil || assessment.Status != "partial" || len(assessment.Matches) != 0 {
		t.Fatal("expired evidence certified current coverage", err)
	}
}

func waitDiskSource(t *testing.T, s *EnhancedService, id string) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		job, err := s.SourceTest(id)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status == "completed" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("disk source did not settle")
}
