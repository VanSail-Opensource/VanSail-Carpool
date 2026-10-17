package iplookup

import "net/netip"

// SetDetectedPrefixes publishes the internal evidence overlay in memory. Paid
// exemptions change enforcement only and deliberately leave this evidence intact.
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
	pool.sources = append(pool.sources, ProxySourceResult{ID: "xnode-detected", Name: "X-Node", Status: "success", Category: ProxyDatabaseCategory, Count: index.count()})
}
