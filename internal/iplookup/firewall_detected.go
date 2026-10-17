package iplookup

import "net/netip"

// SetDetectedPrefixes publishes the effective restriction overlay in memory.
// Historical evidence remains in the existing operational audit records.
func (s *EnhancedService) SetDetectedPrefixes(prefixes []string) {
	builder := newPrefixIndexBuilder(1000000)
	for _, raw := range prefixes {
		if prefix, err := netip.ParsePrefix(raw); err == nil {
			builder.add(prefix)
		}
	}
	index := builder.freeze()
	s.mu.Lock()
	previous := s.detected
	s.detected = index
	s.mu.Unlock()
	previous.release()
}
func (s *EnhancedService) appendDetected(pool *proxyPool) {
	s.mu.Lock()
	index := s.detected.retain()
	s.mu.Unlock()
	if index == nil {
		return
	}
	pool.indexes = append(pool.indexes, index)
	pool.sources = append(append([]ProxySourceResult{}, pool.sources...), ProxySourceResult{ID: "xnode-detected", Name: "X-Node", Status: "success", Category: AbuseCategory, Count: index.count()})
}

func (s *EnhancedService) detectedPool(pool *proxyPool) *proxyPool {
	if pool != nil {
		s.appendDetected(pool)
	}
	return pool
}
