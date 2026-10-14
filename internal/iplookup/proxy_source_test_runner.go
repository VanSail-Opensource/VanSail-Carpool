package iplookup

import (
	"context"
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
		result.Category = sourceCategory(cache.source.Category)
		b.entries[job.id] = sourceTestEntry{test: ProxySourceTest{ID: job.id, Status: "processing", Result: result}}
		if b.sources[job.source.ID] == cache {
			cache.result = result
		}
	}
	prefixes, result := s.collectSourceWithLimit(collectCtx, job.source, c, true, update, job.entryLimit)
	if collectCtx.Err() != nil {
		result.Status, result.ErrorCode = "partial", "IP_PROXY_INTERRUPTED"
	}
	s.finishSource(b, job, prefixes, result)
}

func interruptedSource(source ProxySource) ProxySourceResult {
	return ProxySourceResult{ID: source.ID, Name: source.Name, Category: sourceCategory(source.Category), URL: safeSourceURL(source.URL), Status: "partial", Phase: "complete", ErrorCode: "IP_PROXY_INTERRUPTED"}
}

func (s *EnhancedService) finishSource(b *sourceTestRuntime, job sourceRefreshJob, index *prefixIndex, result ProxySourceResult) {
	cache := job.cache
	b.mu.Lock()
	previous := cache.index
	if !cache.expires.After(time.Now()) {
		previous = nil
		cache.index, cache.complete = nil, false
		b.revision++
	}
	b.mu.Unlock()
	if result.Status != "success" {
		index = mergePrefixIndexes(previous, index, job.entryLimit)
	}
	b.mu.Lock()
	result.Name = cache.source.Name
	result.Category = sourceCategory(cache.source.Category)
	now := time.Now()
	if result.ObservedAt == "" {
		result.ObservedAt = now.UTC().Format(time.RFC3339)
	}
	if b.sources[job.source.ID] == cache {
		cache.index, cache.result, cache.complete = index, result, result.Status == "success"
		cache.result.Count = index.count()
		b.revision++
		// Partial observations have the same bounded retention window but never
		// certify coverage. Retaining a fresh fallback does not extend its TTL.
		if cache.complete || previous == nil {
			cache.expires = now.Add(proxyInventoryTTL)
		}
		cache.nextRefresh = now.Add(proxyInventoryTTL)
		result = cache.result
	}
	b.mu.Unlock()
	b.refreshCount()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[job.id] = sourceTestEntry{test: ProxySourceTest{ID: job.id, Status: "completed", Result: result}, expires: now.Add(15 * time.Minute)}
	close(job.done)
	cache.done, cache.cancel, cache.automatic = nil, nil, false
}
