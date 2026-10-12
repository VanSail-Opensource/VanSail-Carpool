package iplookup

import (
	"context"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type dnsResolution struct {
	ips          []netip.Addr
	status       string
	ttl          time.Duration
	observations []DNSObservation
}
type dnsEntry struct {
	result  dnsResolution
	expires time.Time
}

func (s *EnhancedService) resolve(ctx context.Context, host string, servers []string, all bool) ([]netip.Addr, error) {
	r := s.resolveDNS(ctx, host, servers, 0, false, all)
	if r.status == "error" {
		return r.ips, &CodeError{"IP_DNS_UNAVAILABLE"}
	}
	return r.ips, nil
}

// resolveDNS uses one complete resolver for inventories and independent evidence for Reality.
func (s *EnhancedService) resolveDNS(ctx context.Context, host string, servers []string, family uint16, fresh, collect bool) dnsResolution {
	if ip, err := netip.ParseAddr(host); err == nil {
		if _, err := CanonicalIP(ip.String()); err != nil {
			return dnsResolution{status: "no_address"}
		}
		return dnsResolution{ips: []netip.Addr{ip.Unmap()}, status: "success"}
	}
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	key := strings.Join(servers, ",") + ":" + host
	if !fresh && !collect {
		s.mu.Lock()
		cached := s.dns[key]
		s.mu.Unlock()
		if cached.expires.After(time.Now()) {
			return cached.result
		}
	}
	r := dnsResolution{status: "error", ttl: 30 * time.Minute, observations: []DNSObservation{}}
	seen := map[netip.Addr]bool{}
	for _, server := range s.resolverOrder(servers) {
		one := s.resolveAt(ctx, host, server, family)
		obs := DNSObservation{Server: server, Status: one.status, Addresses: []string{}}
		for _, ip := range one.ips {
			seen[ip] = true
			obs.Addresses = append(obs.Addresses, ip.String())
		}
		sort.Strings(obs.Addresses)
		r.observations = append(r.observations, obs)
		if one.status != "error" {
			r.status = one.status
			r.ttl = min(r.ttl, one.ttl)
			if !collect {
				break
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	for ip := range seen {
		r.ips = append(r.ips, ip)
	}
	sort.Slice(r.ips, func(i, j int) bool { return r.ips[i].Less(r.ips[j]) })
	if len(r.ips) > 0 && r.status != "error" {
		r.status = "success"
	}
	if !collect && r.status != "error" && r.ttl > 0 {
		s.mu.Lock()
		if len(s.dns) >= 32768 {
			for k, v := range s.dns {
				if !v.expires.After(time.Now()) {
					delete(s.dns, k)
				}
			}
		}
		if len(s.dns) < 32768 {
			s.dns[key] = dnsEntry{result: r, expires: time.Now().Add(r.ttl)}
		}
		s.mu.Unlock()
	}
	return r
}

func (s *EnhancedService) resolveAt(ctx context.Context, host, server string, family uint16) dnsResolution {
	r := dnsResolution{status: "success", ttl: 30 * time.Minute}
	kinds := []uint16{dns.TypeA, dns.TypeAAAA}
	if family != 0 {
		kinds = []uint16{family}
	}
	for _, kind := range kinds {
		answer, err := s.dnsQuery(ctx, host, server, kind, 0)
		if err != nil {
			r.status = "error"
			continue
		}
		negative := time.Minute
		for _, rr := range answer.Ns {
			if soa, ok := rr.(*dns.SOA); ok {
				negative = min(time.Duration(min(soa.Hdr.Ttl, soa.Minttl))*time.Second, 5*time.Minute)
			}
		}
		if answer.Rcode == dns.RcodeNameError {
			return dnsResolution{status: "no_address", ttl: negative}
		}
		found := false
		for _, rr := range answer.Answer {
			var raw string
			switch value := rr.(type) {
			case *dns.A:
				raw = value.A.String()
			case *dns.AAAA:
				raw = value.AAAA.String()
			}
			if ip, err := netip.ParseAddr(raw); err == nil {
				found = true
				r.ttl = min(r.ttl, time.Duration(rr.Header().Ttl)*time.Second)
				if _, err := CanonicalIP(ip.String()); err == nil {
					r.ips = append(r.ips, ip.Unmap())
				}
			}
		}
		if !found {
			r.ttl = min(r.ttl, negative)
		}
	}
	if r.status != "error" && len(r.ips) == 0 {
		r.status = "no_address"
	}
	return r
}
