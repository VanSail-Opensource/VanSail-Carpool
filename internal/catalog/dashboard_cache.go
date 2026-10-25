package catalog

import (
	"encoding/json"
	"time"
)

const dashboardCacheEntries = 256
const dashboardCacheBytes = 8 << 20

// Only the existing owner-scoped usage snapshot is retained. Financial and
// entitlement reads still precede every cache lookup in Dashboard.
func (s *Service) cacheDashboard(owner string, value RemoteDashboard, now time.Time) {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > dashboardCacheBytes {
		s.InvalidateDashboard(owner)
		return
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if s.cache == nil {
		s.cache = map[string]cachedDashboard{}
	}
	s.cacheBytes -= s.cache[owner].bytes
	delete(s.cache, owner)
	for len(s.cache) > 0 && (len(s.cache) >= dashboardCacheEntries || s.cacheBytes+len(raw) > dashboardCacheBytes) {
		oldest := ""
		var at time.Time
		for id, entry := range s.cache {
			if oldest == "" || entry.fetchedAt.Before(at) {
				oldest, at = id, entry.fetchedAt
			}
		}
		s.cacheBytes -= s.cache[oldest].bytes
		delete(s.cache, oldest)
	}
	s.cache[owner] = cachedDashboard{value: value, fetchedAt: now, bytes: len(raw)}
	s.cacheBytes += len(raw)
}
