package iplookup

import (
	"context"
	"time"
)

// pruneInventoryMemos releases expired DNS/pool references during maintenance,
// even when no member requests arrive or source configuration cannot be read.
func (s *EnhancedService) pruneInventoryMemos() {
	if s.disk != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		_ = s.disk.prune(ctx)
		cancel()
	}
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
			pool.release()
			delete(s.pools, key)
		}
	}
}
