package iplookup

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestSourceCategoriesValidateAndPreserveLegacy(t *testing.T) {
	for _, category := range []ProxySourceCategory{"", ProxyPoolCategory, ProxyDatabaseCategory, AbuseCategory, "invalid"} {
		t.Run(string(category), func(t *testing.T) {
			c := DefaultConfig()
			c.ProxySources = []ProxySource{{ID: "source", Name: "Source", URL: "https://example.net/list", Enabled: true, Category: category}}
			err := normalizeEnhancedConfig(&c)
			if (err != nil) != (category == "invalid") {
				t.Fatalf("category=%q error=%v", category, err)
			}
			if category == "" && c.ProxySources[0].Category != ProxyPoolCategory {
				t.Fatal("legacy default missing")
			}
		})
	}
	if DefaultProxySources()[0].Category != ProxyDatabaseCategory {
		t.Fatal("new ProxyDB default incorrect")
	}
	r, err := DecodeReport([]byte(`{"id":"legacy","proxy":{"status":"success","sources":[{"id":"old","name":"Old"}],"matches":["old"]},"refusals":[]}`))
	if err != nil || r.Proxy.Sources[0].Category != ProxyPoolCategory {
		t.Fatalf("historical category: %+v err=%v", r.Proxy, err)
	}
}

func TestCategoryMatchesProduceDistinctIdempotentRefusals(t *testing.T) {
	r := Report{Verdict: "suitable", Proxy: &ProxyAssessment{Status: "success", Sources: []ProxySourceResult{
		{ID: "pool", Name: "Pool", Category: ProxyPoolCategory},
		{ID: "database", Name: "Database", Category: ProxyDatabaseCategory},
		{ID: "abuse", Name: "Abuse", Category: AbuseCategory},
	}, Matches: []string{"pool", "database", "abuse"}}}
	applyEnhancedVerdict(&r)
	applyEnhancedVerdict(&r)
	kinds := []string{}
	for _, refusal := range r.Refusals {
		kinds = append(kinds, refusal.Kind)
	}
	if r.Verdict != "unsuitable" || !reflect.DeepEqual(kinds, []string{"proxy_list", "proxy_database", "abuse_list"}) {
		t.Fatalf("incorrect categories: %+v", r.Refusals)
	}
}

func TestSourceCategoryAndNameEditsReusePublishedIndex(t *testing.T) {
	s := diagnosticService(t)
	c := DefaultConfig()
	c.ProxySources = []ProxySource{{ID: "source", Name: "Old", URL: "https://example.net/list", Enabled: true}}
	settings := &sourceSettings{}
	settings.set(t, c)
	s.settings = settings
	var calls atomic.Int32
	s.inventoryFetch = func(context.Context, string, Config) ([]byte, error) { calls.Add(1); return []byte("11.0.0.0/24"), nil }
	ctx, cancel := context.WithCancel(context.Background())
	s.StartSourceTests(ctx)
	t.Cleanup(func() { cancel(); s.WaitSourceTests() })
	awaitSource(t, s, "source")
	s.tests.mu.Lock()
	previous := s.tests.sources["source"].index
	s.tests.mu.Unlock()
	c.ProxySources[0].Name, c.ProxySources[0].Category = "Abuse feed", AbuseCategory
	settings.set(t, c)
	if err := s.ReconcileSources(ctx); err != nil {
		t.Fatal(err)
	}
	pool, err := s.pool(ctx, c)
	if err != nil || calls.Load() != 1 || pool.indexes[0] != previous || pool.sources[0].Category != AbuseCategory || pool.sources[0].Name != "Abuse feed" {
		t.Fatalf("metadata edit collected again: pool=%+v err=%v", pool, err)
	}
	r := Report{Proxy: pool.assessment("11.0.0.1")}
	applyProxyRefusal(&r)
	if len(r.Refusals) != 1 || r.Refusals[0].Kind != "abuse_list" {
		t.Fatal("new category did not affect future evidence")
	}
	c.ProxySources[0].Enabled = false
	settings.set(t, c)
	if err := s.ReconcileSources(ctx); err != nil {
		t.Fatal(err)
	}
	status, err := s.SourcesStatus(ctx)
	if err != nil || status.UniqueIPs != 0 {
		t.Fatal("disabled source remained in cached count")
	}
}
