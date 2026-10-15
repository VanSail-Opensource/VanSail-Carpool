package iplookup

import (
	"bytes"
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
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	return u.String()
}

func (s *EnhancedService) buildPool(ctx context.Context, c Config) (*proxyPool, error) {
	p := &proxyPool{sources: []ProxySourceResult{}, complete: true, expires: time.Now().Add(proxyInventoryTTL)}
	complete := false
	defer func() {
		if !complete {
			p.release()
		}
	}()
	for _, source := range c.ProxySources {
		if !source.Enabled {
			continue
		}
		index, result := s.collectSourceWithLimit(ctx, source, c, false, nil, proxySourceEntryLimit(len(c.ProxySources)))
		if ctx.Err() != nil {
			index.release()
			return nil, ctx.Err()
		}
		p.indexes = append(p.indexes, index)
		p.sources = append(p.sources, result)
		p.complete = p.complete && result.Status == "success"
	}
	if len(p.sources) == 0 {
		p.complete = false
	}
	complete = true
	return p, nil
}

func (s *EnhancedService) collectSource(ctx context.Context, source ProxySource, c Config, fresh bool, emit func(ProxySourceResult)) (*prefixIndex, ProxySourceResult) {
	return s.collectSourceWithLimit(ctx, source, c, fresh, emit, maxInventoryEntries)
}

func (s *EnhancedService) collectSourceWithLimit(ctx context.Context, source ProxySource, c Config, fresh bool, emit func(ProxySourceResult), entryLimit int) (*prefixIndex, ProxySourceResult) {
	r := ProxySourceResult{ID: source.ID, Name: source.Name, Category: sourceCategory(source.Category), URL: safeSourceURL(source.URL), Status: "processing", Phase: "download"}
	var last time.Time
	update := func(force bool) {
		if emit != nil && (force || time.Since(last) >= 250*time.Millisecond) {
			emit(r)
			last = time.Now()
		}
	}
	update(true)
	builder := s.newInventoryBuilder(ctx, entryLimit)
	add := func(host string) {
		if ctx.Err() != nil {
			return
		}
		if r.Phase == "download" {
			r.Phase = "parse"
			update(true)
		}
		prefix, err := netip.ParsePrefix(host)
		if err != nil {
			if ip, err := netip.ParseAddr(host); err == nil {
				prefix = netip.PrefixFrom(ip.Unmap(), ip.Unmap().BitLen())
			}
		}
		if prefix.IsValid() {
			before := builder.index.count()
			if !builder.add(prefix) {
				r.Status, r.ErrorCode = "partial", "IP_PROXY_LIMIT"
			}
			r.Hosts += builder.index.count() - before
			r.Processed += builder.index.count() - before
			r.Count = builder.index.count()
		} else {
			before := builder.domainCount()
			if !builder.addDomain(host) {
				r.Status, r.ErrorCode = "partial", "IP_PROXY_LIMIT"
				return
			}
			r.Hosts += builder.domainCount() - before
		}
		update(false)
	}
	var err error
	if builder.err != nil {
		err = builder.err
	} else if s.inventoryFetch != nil {
		var data []byte
		data, err = s.inventoryFetch(ctx, source.URL, c)
		if err == nil {
			r.Phase = "parse"
			update(true)
			err = readInventory(ctx, bytes.NewReader(data), add)
		}
	} else if s.inventoryStream != nil {
		err = s.inventoryStream(ctx, source.URL, c, add)
	} else {
		err = s.downloadInventory(ctx, source.URL, c, add)
	}
	builder.flush()
	r.Hosts = builder.index.count() + builder.domainCount()
	r.Processed = builder.index.count()
	if builder.err != nil {
		err = &CodeError{inventoryBuildError(builder.err)}
	}
	if err != nil {
		r.Status, r.ErrorCode = "partial", ErrorCode(err)
		if r.ErrorCode != "IP_PROXY_PARSE" && r.ErrorCode != "IP_PROXY_LIMIT" && r.ErrorCode != "IP_LOOKUP_FAILED" && r.ErrorCode != "IP_LOOKUP_UNAVAILABLE" {
			r.Status, r.ErrorCode = "error", "IP_PROXY_DOWNLOAD"
		}
	} else if r.Status == "processing" {
		r.Status = "success"
	}
	if r.Hosts == 0 && err == nil {
		r.Status, r.ErrorCode = "partial", "IP_PROXY_EMPTY"
	}
	// Resolve domains after download/parsing, outside the proxy_lists queue.
	if builder.domainCount() > 0 && ctx.Err() == nil && builder.err == nil {
		r.Phase = "dns"
		update(true)
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
						if !builder.add(netip.PrefixFrom(ip, ip.BitLen())) {
							r.Status, r.ErrorCode = "partial", "IP_PROXY_LIMIT"
						}
					}
					r.Count = builder.index.count()
					update(false)
					mu.Unlock()
				}
			}()
		}
		domainErr := builder.eachDomain(ctx, func(host string) bool {
			select {
			case jobs <- host:
				return true
			case <-ctx.Done():
				return false
			}
		})
		close(jobs)
		work.Wait()
		if domainErr != nil {
			builder.err = domainErr
		}
	}
	if ctx.Err() != nil {
		r.Status, r.ErrorCode = "partial", "IP_PROXY_INTERRUPTED"
	}
	index := builder.freeze()
	if builder.err != nil && ctx.Err() == nil {
		r.Status, r.ErrorCode = "partial", inventoryBuildError(builder.err)
	}
	r.Count = index.count()
	r.Phase, r.ObservedAt = "complete", time.Now().UTC().Format(time.RFC3339)
	update(true)
	return index, r
}
