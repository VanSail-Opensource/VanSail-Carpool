package iplookup

import (
	"context"
	"net/netip"
	"net/url"
	"sync"
	"time"
)

const proxyInventoryTTL = 6 * time.Hour

func safeSourceURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func (s *EnhancedService) buildPool(ctx context.Context, c Config) (*proxyPool, error) {
	p := &proxyPool{ips: map[netip.Addr][]string{}, sources: []ProxySourceResult{}, complete: true}
	for _, source := range c.ProxySources {
		if !source.Enabled {
			continue
		}
		ips, result := s.collectSource(ctx, source, c, false, nil)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		for ip := range ips {
			if len(p.ips) >= 1000000 && p.ips[ip] == nil {
				result.Status = "partial"
				result.ErrorCode = "IP_PROXY_LIMIT"
				continue
			}
			p.ips[ip] = append(p.ips[ip], source.ID)
		}
		p.sources = append(p.sources, result)
		p.complete = p.complete && result.Status == "success"
	}
	if len(p.sources) == 0 {
		p.complete = false
	}
	p.expires = time.Now().Add(proxyInventoryTTL)
	return p, nil
}

func (s *EnhancedService) collectSource(ctx context.Context, source ProxySource, c Config, fresh bool, emit func(ProxySourceResult)) (map[netip.Addr]bool, ProxySourceResult) {
	r := ProxySourceResult{ID: source.ID, Name: source.Name, URL: safeSourceURL(source.URL), Status: "processing", Phase: "download"}
	update := func() {
		if emit != nil {
			emit(r)
		}
	}
	update()
	ips := map[netip.Addr]bool{}
	fetch := s.downloadInventory
	if s.inventoryFetch != nil {
		fetch = s.inventoryFetch
	}
	data, err := fetch(ctx, source.URL, c)
	if err != nil {
		r.Status = "error"
		r.ErrorCode = "IP_PROXY_DOWNLOAD"
		update()
		return ips, r
	}
	r.Phase = "parse"
	update()
	hosts, err := ExtractProxyHosts(data)
	r.Status = "success"
	if err != nil {
		r.Status = "partial"
		r.ErrorCode = "IP_PROXY_PARSE"
	}
	if len(hosts) == 0 {
		r.Status = "partial"
		r.ErrorCode = "IP_PROXY_EMPTY"
	}
	r.Hosts = len(hosts)
	r.Phase = "dns"
	update()
	var mu sync.Mutex
	var work sync.WaitGroup
	jobs := make(chan string)
	for range 4 {
		work.Add(1)
		go func() {
			defer work.Done()
			for host := range jobs {
				resolved := s.resolveDNS(ctx, host, c.DNSResolvers, 0, fresh, false)
				mu.Lock()
				r.Processed++
				if resolved.status == "error" {
					r.DNSFailed++
				}
				if resolved.status == "error" || resolved.status == "no_address" {
					r.Skipped++
				}
				for _, ip := range resolved.ips {
					if len(ips) >= 1000000 && !ips[ip] {
						r.Status, r.ErrorCode = "partial", "IP_PROXY_LIMIT"
						continue
					}
					ips[ip] = true
				}
				r.Count = len(ips)
				update()
				mu.Unlock()
			}
		}()
	}
	for _, host := range hosts {
		if ctx.Err() != nil {
			break
		}
		select {
		case jobs <- host:
		case <-ctx.Done():
		}
	}
	close(jobs)
	work.Wait()
	if ctx.Err() != nil {
		r.Status = "partial"
		r.ErrorCode = "IP_PROXY_INTERRUPTED"
	}
	r.Phase = "complete"
	r.ObservedAt = time.Now().UTC().Format(time.RFC3339)
	update()
	return ips, r
}
