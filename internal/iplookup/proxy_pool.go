package iplookup

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/netip"
	"sync"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

type proxyPool struct {
	ips      map[netip.Addr][]string
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
	settings Settings
	queues   map[string]*upstreamqueue.Queue
	live     *LiveService
	mu       sync.Mutex
	pools    map[[32]byte]*proxyPool
	flights  map[[32]byte]*poolFlight
	dns      map[string]dnsEntry
	ipapi    Provider
}

// NewEnhancedService reuses existing queue workers for every outbound operation.
func NewEnhancedService(settings Settings, queues map[string]*upstreamqueue.Queue, live *LiveService) *EnhancedService {
	return &EnhancedService{settings: settings, queues: queues, live: live, pools: map[[32]byte]*proxyPool{}, flights: map[[32]byte]*poolFlight{}, dns: map[string]dnsEntry{}, ipapi: NewHTTPProvider("ipapi", queues["ipapi"], settings)}
}
func proxyPoolKey(c Config) [32]byte {
	raw, _ := json.Marshal(struct {
		Sources []ProxySource
		DNS     []string
	}{c.ProxySources, c.DNSResolvers})
	return sha256.Sum256(raw)
}
func (s *EnhancedService) pool(ctx context.Context, c Config) (*proxyPool, error) {
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
func (s *EnhancedService) buildPool(ctx context.Context, c Config) (*proxyPool, error) {
	p := &proxyPool{ips: map[netip.Addr][]string{}, sources: []ProxySourceResult{}, complete: true, expires: time.Now().Add(30 * time.Minute)}
	for _, source := range c.ProxySources {
		if !source.Enabled {
			continue
		}
		result := ProxySourceResult{ID: source.ID, Name: source.Name, Status: "success", ObservedAt: time.Now().UTC().Format(time.RFC3339)}
		data, err := s.downloadInventory(ctx, source.URL, c)
		hosts := []string{}
		if err == nil {
			hosts, err = ExtractProxyHosts(data)
		}
		if err != nil {
			result.Status = "partial"
			p.complete = false
		}
		var mu sync.Mutex
		var work sync.WaitGroup
		jobs := make(chan string)
		for range 4 {
			work.Add(1)
			go func() {
				defer work.Done()
				for host := range jobs {
					ips, err := s.resolve(ctx, host, c.DNSResolvers, false)
					mu.Lock()
					if err != nil {
						result.Status = "partial"
						p.complete = false
					}
					for _, ip := range ips {
						if _, e := CanonicalIP(ip.String()); e != nil {
							continue
						}
						if len(p.ips) >= 1000000 {
							result.Status = "partial"
							p.complete = false
							continue
						}
						found := false
						for _, id := range p.ips[ip] {
							found = found || id == source.ID
						}
						if !found {
							p.ips[ip] = append(p.ips[ip], source.ID)
							result.Count++
						}
					}
					mu.Unlock()
				}
			}()
		}
		for _, host := range hosts {
			select {
			case jobs <- host:
			case <-ctx.Done():
				close(jobs)
				work.Wait()
				return nil, ctx.Err()
			}
		}
		close(jobs)
		work.Wait()
		if len(hosts) == 0 {
			result.Status = "partial"
			p.complete = false
		}
		p.sources = append(p.sources, result)
	}
	if len(p.sources) == 0 {
		p.complete = false
	}
	return p, nil
}
func (p *proxyPool) assessment(ip string) *ProxyAssessment {
	addr, _ := netip.ParseAddr(ip)
	matches := append([]string{}, p.ips[addr.Unmap()]...)
	status := "success"
	if !p.complete {
		status = "partial"
	}
	return &ProxyAssessment{Status: status, Sources: append([]ProxySourceResult{}, p.sources...), Matches: matches}
}
