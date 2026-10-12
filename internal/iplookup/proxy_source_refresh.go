package iplookup

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/ids"
)

func sourceCacheKey(source ProxySource, c Config) [32]byte {
	raw, _ := json.Marshal(struct {
		URL string
		DNS []string
	}{source.URL, c.DNSResolvers})
	return sha256.Sum256(raw)
}

func (s *EnhancedService) reconcileSourcesLocked(b *sourceTestRuntime, c Config) {
	now := time.Now()
	for id, entry := range b.entries {
		if !entry.expires.IsZero() && !entry.expires.After(now) {
			delete(b.entries, id)
		}
	}
	seen := map[string]bool{}
	for _, source := range c.ProxySources {
		seen[source.ID] = true
		key := sourceCacheKey(source, c)
		cache := b.sources[source.ID]
		if cache == nil || cache.key != key || cache.source.Enabled != source.Enabled {
			if cache != nil && cache.cancel != nil {
				cache.cancel()
			}
			cache = &sourceCache{source: source, key: key, result: ProxySourceResult{ID: source.ID, Name: source.Name, Category: sourceCategory(source.Category), URL: safeSourceURL(source.URL), Status: "pending"}}
			b.sources[source.ID] = cache
			b.revision++
		} else {
			cache.source.Name, cache.result.Name = source.Name, source.Name
			cache.source.Category, cache.result.Category = source.Category, sourceCategory(source.Category)
		}
		if source.Enabled && cache.done == nil && !cache.nextRefresh.After(now) {
			_, _ = s.startSourceLocked(b, cache, c) // Retry admission failures on the next tick.
		}
	}
	for id, cache := range b.sources {
		if !seen[id] {
			if cache.cancel != nil {
				cache.cancel()
			}
			delete(b.sources, id)
			b.revision++
		}
	}
}

func (s *EnhancedService) startSourceLocked(b *sourceTestRuntime, cache *sourceCache, c Config) (ProxySourceTest, error) {
	if cache.done != nil {
		return b.entries[cache.jobID].test, nil
	}
	if len(b.entries) >= 128 {
		return ProxySourceTest{}, &CodeError{"IP_LOOKUP_BUSY"}
	}
	id, err := ids.New()
	if err != nil {
		return ProxySourceTest{}, err
	}
	ctx, cancel := context.WithCancel(b.ctx)
	cache.jobID, cache.done, cache.cancel = id, make(chan struct{}), cancel
	cache.result.Status, cache.result.Phase, cache.result.ErrorCode = "pending", "download", ""
	cache.result.Category = sourceCategory(cache.source.Category)
	test := ProxySourceTest{ID: id, Status: "queued", Result: cache.result}
	b.entries[id] = sourceTestEntry{test: test}
	b.work.Add(1)
	job := sourceRefreshJob{cache: cache, source: cache.source, id: id, done: cache.done, cancel: cancel}
	go s.runSourceTest(b, job, c, ctx)
	return test, nil
}
