package iplookup

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"
)

var inventoryBenchmarkSink any

func BenchmarkProxyInventoryStatus(b *testing.B) {
	for _, entries := range []int{100, 3000000} {
		name := "small"
		if entries > 100 {
			name = "large"
		}
		b.Run(name, func(b *testing.B) {
			builder := newPrefixIndexBuilder(maxInventoryEntries)
			for i := range entries {
				builder.add(netip.PrefixFrom(generatedInventoryIP(i), 32))
			}
			c := DefaultConfig()
			c.ProxySources = []ProxySource{{ID: "one", Name: "One", URL: "https://example.net/list", Enabled: true}}
			raw, err := json.Marshal(c)
			if err != nil {
				b.Fatal(err)
			}
			s := NewEnhancedService(&sourceSettings{value: string(raw)}, nil, nil)
			s.tests = &sourceTestRuntime{sources: map[string]*sourceCache{"one": {source: c.ProxySources[0], key: sourceCacheKey(c.ProxySources[0], c), index: builder.freeze()}}, uniqueEntries: entries}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				result, err := s.SourcesStatus(context.Background())
				if err != nil {
					b.Fatal(err)
				}
				inventoryBenchmarkSink = result
			}
		})
	}
}

func BenchmarkProxyInventoryMemory(b *testing.B) {
	const entries = 3000000
	b.Run("compressed", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			builder := newPrefixIndexBuilder(maxInventoryEntries)
			for i := range entries {
				builder.add(netip.PrefixFrom(generatedInventoryIP(i), 32))
			}
			inventoryBenchmarkSink = builder.freeze()
		}
	})
	b.Run("previous_map", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			entriesMap := make(map[netip.Prefix]bool, entries)
			for i := range entries {
				entriesMap[netip.PrefixFrom(generatedInventoryIP(i), 32)] = true
			}
			inventoryBenchmarkSink = entriesMap
		}
	})
}

func BenchmarkProxyInventoryLookup(b *testing.B) {
	for _, entries := range []int{100, 3000000} {
		name := "small"
		if entries > 100 {
			name = "large"
		}
		b.Run(name, func(b *testing.B) {
			builder := newPrefixIndexBuilder(maxInventoryEntries)
			for i := range entries {
				builder.add(netip.PrefixFrom(generatedInventoryIP(i), 32))
			}
			pool := &proxyPool{indexes: []*prefixIndex{builder.freeze()}, sources: []ProxySourceResult{{ID: "one"}}, complete: true}
			ip := generatedInventoryIP(entries - 1)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				inventoryBenchmarkSink = pool.matchingSources(ip)
			}
		})
	}
}
