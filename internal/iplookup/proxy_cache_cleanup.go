package iplookup

import "time"

// pruneInventoryMemos releases expired DNS/pool references during maintenance,
// even when no member requests arrive or source configuration cannot be read.
func (s *EnhancedService) pruneInventoryMemos() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entry := range s.dns {
		if !entry.expires.After(now) {
			delete(s.dns, key)
		}
	}
	for key, pool := range s.pools {
		if !pool.expires.After(now) {
			delete(s.pools, key)
		}
	}
}
