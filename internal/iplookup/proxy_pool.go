package iplookup

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/netip"
	"sort"
	"sync"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

type proxyPool struct {
	prefixes map[netip.Prefix][]string
	sources  []ProxySourceResult
	complete bool
	expires  time.Time
}
type poolFlight struct {
	done chan struct{}
	pool *proxyPool
	err  error
}

// EnhancedService owns shared inventories, DNS memoization and queued probes.
type EnhancedService struct {
	settings       Settings
	queues         map[string]*upstreamqueue.Queue
	live           *LiveService
	mu             sync.Mutex
	pools          map[[32]byte]*proxyPool
	flights        map[[32]byte]*poolFlight
	dns            map[string]dnsEntry
	ipapi          Provider
	inventoryFetch func(context.Context, string, Config) ([]byte, error)
	resolvers      map[string]resolverHealth
	cursor         uint64
	tests          *sourceTestRuntime
}

// NewEnhancedService reuses existing queue workers for every outbound operation.
func NewEnhancedService(settings Settings, queues map[string]*upstreamqueue.Queue, live *LiveService) *EnhancedService {
	return &EnhancedService{settings: settings, queues: queues, live: live, pools: map[[32]byte]*proxyPool{}, flights: map[[32]byte]*poolFlight{}, dns: map[string]dnsEntry{}, resolvers: map[string]resolverHealth{}, ipapi: NewHTTPProvider("ipapi", queues["ipapi"], settings)}
}
func proxyPoolKey(c Config) [32]byte {
	raw, _ := json.Marshal(struct {
		Sources []ProxySource
		DNS     []string
	}{c.ProxySources, c.DNSResolvers})
	return sha256.Sum256(raw)
}
func (s *EnhancedService) pool(ctx context.Context, c Config) (*proxyPool, error) {
	if s.tests != nil {
		return s.cachedPool(ctx, c)
	}
	key := proxyPoolKey(c)
	s.mu.Lock()
	cached := s.pools[key]
	if cached != nil && cached.expires.After(time.Now()) {
		s.mu.Unlock()
		return cached, nil
	}
	if f := s.flights[key]; f != nil {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-f.done:
			return f.pool, f.err
		}
	}
	f := &poolFlight{done: make(chan struct{})}
	s.flights[key] = f
	s.mu.Unlock()
	p, err := s.buildPool(ctx, c)
	s.mu.Lock()
	f.pool, f.err = p, err
	if err == nil {
		for k, v := range s.pools {
			if !v.expires.After(time.Now()) {
				delete(s.pools, k)
			}
		}
		if len(s.pools) >= 4 {
			clear(s.pools)
		}
		s.pools[key] = p
	}
	delete(s.flights, key)
	close(f.done)
	s.mu.Unlock()
	return p, err
}
func (p *proxyPool) assessment(ip string) *ProxyAssessment {
	addr, _ := netip.ParseAddr(ip)
	matches := p.matchingSources(addr)
	status := "success"
	if !p.complete {
		status = "partial"
	}
	return &ProxyAssessment{Status: status, Sources: append([]ProxySourceResult{}, p.sources...), Matches: matches}
}

func (p *proxyPool) matchingSources(addr netip.Addr) []string {
	matches := []string{}
	seen := map[string]bool{}
	addr = addr.Unmap()
	// At most 129 map lookups, regardless of inventory size or CIDR capacity.
	for bits := addr.BitLen(); bits >= 0 && addr.IsValid(); bits-- {
		for _, id := range p.prefixes[netip.PrefixFrom(addr, bits).Masked()] {
			if !seen[id] {
				seen[id] = true
				matches = append(matches, id)
			}
		}
	}
	sort.Strings(matches)
	return matches
}
