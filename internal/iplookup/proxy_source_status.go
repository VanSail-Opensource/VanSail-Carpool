package iplookup

import (
	"context"
	"net/netip"
	"time"
)

// ProxySourceCacheStatus describes collection and cache freshness.
type ProxySourceCacheStatus struct {
	ProxySourceResult
	Enabled       bool   `json:"enabled"`
	Refreshing    bool   `json:"refreshing"`
	Stale         bool   `json:"stale"`
	ExpiresAt     string `json:"expiresAt"`
	NextRefreshAt string `json:"nextRefreshAt"`
}

// ProxySourcesStatus is an administrator-only inventory summary.
type ProxySourcesStatus struct {
	Sources   []ProxySourceCacheStatus `json:"sources"`
	UniqueIPs int                      `json:"uniqueIPs"`
}

func sourceTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

// SourcesStatus reads snapshots without triggering external requests.
func (s *EnhancedService) SourcesStatus(ctx context.Context) (ProxySourcesStatus, error) {
	c, err := s.sourceConfig(ctx)
	if err != nil {
		return ProxySourcesStatus{}, err
	}
	result := ProxySourcesStatus{Sources: []ProxySourceCacheStatus{}}
	b := s.tests
	if b == nil {
		return result, &CodeError{"IP_LOOKUP_UNAVAILABLE"}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	unique := map[netip.Addr]bool{}
	for _, source := range c.ProxySources {
		row := ProxySourceCacheStatus{ProxySourceResult: ProxySourceResult{ID: source.ID, Name: source.Name, Status: "pending"}, Enabled: source.Enabled, Stale: true}
		if cache := b.sources[source.ID]; cache != nil && cache.key == sourceCacheKey(source, c) {
			row.ProxySourceResult = cache.result
			row.Count = len(cache.ips)
			row.Refreshing = cache.done != nil
			row.Stale = !cache.complete || !cache.expires.After(time.Now())
			row.ExpiresAt, row.NextRefreshAt = sourceTime(cache.expires), sourceTime(cache.nextRefresh)
			if source.Enabled {
				for ip := range cache.ips {
					unique[ip] = true
				}
			}
		}
		row.Name = source.Name
		if !source.Enabled {
			row.Status = "disabled"
		}
		result.Sources = append(result.Sources, row)
	}
	result.UniqueIPs = len(unique)
	return result, nil
}

func (s *EnhancedService) cachedPool(ctx context.Context, c Config) (*proxyPool, error) {
	b := s.tests
	b.mu.Lock()
	waits := []<-chan struct{}{}
	for _, source := range c.ProxySources {
		if cache := b.sources[source.ID]; source.Enabled && cache != nil && cache.key == sourceCacheKey(source, c) && cache.ips == nil && cache.done != nil {
			waits = append(waits, cache.done)
		}
	}
	b.mu.Unlock()
	for _, done := range waits {
		select {
		case <-done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	p := &proxyPool{ips: map[netip.Addr][]string{}, sources: []ProxySourceResult{}, complete: true, expires: time.Now().Add(proxyInventoryTTL)}
	for _, source := range c.ProxySources {
		if !source.Enabled {
			continue
		}
		row := ProxySourceResult{ID: source.ID, Name: source.Name, Status: "partial"}
		cache := b.sources[source.ID]
		if cache != nil && cache.source.Enabled && cache.key == sourceCacheKey(source, c) {
			row = cache.result
			fresh := cache.complete && cache.expires.After(time.Now())
			if fresh {
				row.Status = "success"
			} else {
				row.Status = "partial"
			}
			for ip := range cache.ips {
				if len(p.ips) >= 1000000 && p.ips[ip] == nil {
					row.Status, row.ErrorCode = "partial", "IP_PROXY_LIMIT"
					continue
				}
				p.ips[ip] = append(p.ips[ip], source.ID)
			}
			if cache.expires.Before(p.expires) {
				p.expires = cache.expires
			}
		}
		row.Name = source.Name
		p.sources = append(p.sources, row)
		p.complete = p.complete && row.Status == "success"
	}
	if len(p.sources) == 0 {
		p.complete = false
	}
	return p, nil
}
