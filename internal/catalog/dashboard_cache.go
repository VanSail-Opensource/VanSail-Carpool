package catalog

import (
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const dashboardCacheEntries = 256
const dashboardCacheBytes = 8 << 20

// Only the existing owner-scoped usage snapshot is retained. Financial and
// entitlement reads still precede every cache lookup in Dashboard.
func (s *Service) cacheDashboard(user model.User, value RemoteDashboard, now time.Time, epoch uint64) bool {
	raw, err := json.Marshal(value)
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	// Invalidation also retires in-flight results, including a successful read
	// of the old bearer that completes after credential rotation.
	if epoch != s.cacheEpoch {
		return false
	}
	owner := user.ID
	if s.cache == nil {
		s.cache = map[string]cachedDashboard{}
	}
	s.cacheBytes -= s.cache[owner].bytes
	delete(s.cache, owner)
	if err != nil || len(raw) > dashboardCacheBytes {
		return true
	}
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
	s.cache[owner] = cachedDashboard{value: value, fetchedAt: now, bytes: len(raw), scope: scopeForDashboard(user)}
	s.cacheBytes += len(raw)
	return true
}

func scopeForDashboard(user model.User) dashboardScope {
	scope := dashboardScope{generation: user.AccountGeneration}
	if user.ProxyUserID != nil {
		scope.proxyID = *user.ProxyUserID
	}
	return scope
}

func (s *Service) dashboardSnapshot(user model.User) (cachedDashboard, bool, uint64) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	cached, exists := s.cache[user.ID]
	if exists && cached.scope != scopeForDashboard(user) {
		s.cacheBytes -= cached.bytes
		delete(s.cache, user.ID)
		s.cacheEpoch++
		exists = false
	}
	return cached, exists, s.cacheEpoch
}
