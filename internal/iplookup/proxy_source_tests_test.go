package iplookup

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestSavedDisabledSourceFullTestIsEphemeralAndSerialized(t *testing.T) {
	s := diagnosticService(t)
	c := DefaultConfig()
	c.ProxySources = []ProxySource{{ID: "saved", Name: "Saved", URL: "https://example.net/saved", Enabled: false}}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	s.settings = providerSettings{value: string(raw)}
	ctx, cancel := context.WithCancel(context.Background())
	s.StartSourceTests(ctx)
	t.Cleanup(func() { cancel(); s.WaitSourceTests() })
	release := make(chan struct{})
	entered := make(chan struct{})
	s.inventoryFetch = func(ctx context.Context, url string, _ Config) ([]byte, error) {
		if url != "https://example.net/saved" {
			t.Error("did not use saved URL")
		}
		close(entered)
		select {
		case <-release:
			return []byte("8.8.8.8:443\n1.1.1.1:443\n8.8.8.8:8443"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	job, err := s.TestSource(context.Background(), "saved")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("test did not enter existing queue")
	}
	if _, err := s.TestSource(context.Background(), "saved"); ErrorCode(err) != "IP_LOOKUP_BUSY" {
		t.Fatal("parallel source test admitted")
	}
	close(release)
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		result, err := s.SourceTest(job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status == "completed" {
			if result.Result.Count != 2 || result.Result.Processed != 2 || result.Result.Status != "success" {
				t.Fatalf("incomplete full test: %+v", result)
			}
			if len(s.pools) != 0 {
				t.Fatal("test populated shared inventory cache")
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("source test did not settle")
		case <-tick.C:
		}
	}
	s.tests.mu.Lock()
	entry := s.tests.entries[job.ID]
	entry.expires = time.Now().Add(-time.Second)
	s.tests.entries[job.ID] = entry
	s.tests.mu.Unlock()
	if _, err := s.SourceTest(job.ID); ErrorCode(err) != "IP_PROXY_SOURCE_NOT_FOUND" {
		t.Fatal("expired diagnostic remained readable")
	}
}
