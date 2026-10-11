package iplookup

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

type dnsEntry struct {
	ips     []netip.Addr
	expires time.Time
}

var dnsCursor atomic.Uint64

func (s *EnhancedService) resolve(ctx context.Context, host string, servers []string, all bool) ([]netip.Addr, error) {
	if ip, e := netip.ParseAddr(host); e == nil {
		return []netip.Addr{ip.Unmap()}, nil
	}
	key := strings.Join(servers, ",") + ":" + host
	if !all {
		s.mu.Lock()
		cached, ok := s.dns[key]
		s.mu.Unlock()
		if ok && cached.expires.After(time.Now()) {
			return append([]netip.Addr{}, cached.ips...), nil
		}
	}
	ips, err := upstreamqueue.Do(ctx, s.queues["dns"], func(ctx context.Context) ([]netip.Addr, error) {
		seen := map[netip.Addr]bool{}
		success := 0
		start := int(dnsCursor.Add(1) % uint64(len(servers)))
		for index := range servers {
			server := servers[(start+index)%len(servers)]
			complete := true
			for _, kind := range []uint16{dns.TypeA, dns.TypeAAAA} {
				req := new(dns.Msg)
				req.SetQuestion(dns.Fqdn(host), kind)
				client := &dns.Client{Net: "udp", Timeout: 2 * time.Second}
				answer, _, e := client.ExchangeContext(ctx, req, server)
				if e == nil && answer.Truncated {
					client.Net = "tcp"
					answer, _, e = client.ExchangeContext(ctx, req, server)
				}
				if e != nil || answer == nil || (answer.Rcode != dns.RcodeSuccess && answer.Rcode != dns.RcodeNameError) {
					complete = false
					continue
				}
				for _, rr := range answer.Answer {
					var raw string
					switch v := rr.(type) {
					case *dns.A:
						raw = v.A.String()
					case *dns.AAAA:
						raw = v.AAAA.String()
					}
					if ip, e := netip.ParseAddr(raw); e == nil {
						seen[ip.Unmap()] = true
					}
				}
			}
			if complete {
				success++
			}
			if !all && complete && len(seen) > 0 {
				break
			}
		}
		out := []netip.Addr{}
		for ip := range seen {
			out = append(out, ip)
		}
		if success == 0 || (all && success != len(servers)) {
			return out, fmt.Errorf("DNS resolution unavailable")
		}
		return out, nil
	})
	if err == nil && !all {
		s.mu.Lock()
		if len(s.dns) >= 32768 {
			for k, v := range s.dns {
				if !v.expires.After(time.Now()) {
					delete(s.dns, k)
				}
			}
		}
		if len(s.dns) < 32768 {
			s.dns[key] = dnsEntry{ips: append([]netip.Addr{}, ips...), expires: time.Now().Add(30 * time.Minute)}
		}
		s.mu.Unlock()
	}
	return ips, err
}
