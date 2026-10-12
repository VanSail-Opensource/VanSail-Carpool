package iplookup

import (
	"context"
	"net/netip"
	"time"
)

func (s *EnhancedService) runSourceTest(b *sourceTestRuntime, job sourceRefreshJob, c Config, ctx context.Context) {
	cache := job.cache
	defer b.work.Done()
	defer job.cancel()
	select {
	case b.slots <- struct{}{}:
		defer func() { <-b.slots }()
	case <-ctx.Done():
		s.finishSource(b, job, nil, interruptedSource(job.source))
		return
	}
	// Waiting for bounded admission must not consume the collection deadline.
	collectCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	update := func(result ProxySourceResult) {
		b.mu.Lock()
		defer b.mu.Unlock()
		result.Name = cache.source.Name
		b.entries[job.id] = sourceTestEntry{test: ProxySourceTest{ID: job.id, Status: "processing", Result: result}}
		if b.sources[job.source.ID] == cache {
			cache.result = result
		}
	}
	prefixes, result := s.collectSource(collectCtx, job.source, c, true, update)
	if collectCtx.Err() != nil {
		result.Status, result.ErrorCode = "partial", "IP_PROXY_INTERRUPTED"
	}
	s.finishSource(b, job, prefixes, result)
}

func interruptedSource(source ProxySource) ProxySourceResult {
	return ProxySourceResult{ID: source.ID, Name: source.Name, URL: safeSourceURL(source.URL), Status: "partial", Phase: "complete", ErrorCode: "IP_PROXY_INTERRUPTED"}
}

func (s *EnhancedService) finishSource(b *sourceTestRuntime, job sourceRefreshJob, prefixes map[netip.Prefix]bool, result ProxySourceResult) {
	cache := job.cache
	b.mu.Lock()
	defer b.mu.Unlock()
	result.Name = cache.source.Name
	now := time.Now()
	if result.ObservedAt == "" {
		result.ObservedAt = now.UTC().Format(time.RFC3339)
	}
	if b.sources[job.source.ID] == cache {
		if result.Status != "success" {
			// Retain positive evidence without certifying negative matches.
			prefixes = retainSourcePrefixes(cache.prefixes, prefixes, 1000000)
		}
		cache.prefixes, cache.result, cache.complete = prefixes, result, result.Status == "success"
		cache.result.Count = len(prefixes)
		if cache.complete {
			cache.expires = now.Add(proxyInventoryTTL)
		}
		cache.nextRefresh = now.Add(proxyInventoryTTL)
		result = cache.result
	}
	b.entries[job.id] = sourceTestEntry{test: ProxySourceTest{ID: job.id, Status: "completed", Result: result}, expires: now.Add(15 * time.Minute)}
	close(job.done)
	cache.done = nil
}

func retainSourcePrefixes(previous, incoming map[netip.Prefix]bool, limit int) map[netip.Prefix]bool {
	retained := make(map[netip.Prefix]bool, len(previous))
	// Prior published evidence wins when a partial refresh fills the limit.
	for prefix := range previous {
		retained[prefix] = true
	}
	for prefix := range incoming {
		if len(retained) < limit || retained[prefix] {
			retained[prefix] = true
		}
	}
	return retained
}
