package iplookup

import (
	"context"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

type resolverHealth struct {
	next, cooldown time.Time
	failures       int
}

func (s *EnhancedService) resolverOrder(servers []string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(servers) == 0 {
		return nil
	}
	s.cursor++
	healthy := []string{}
	for _, server := range servers {
		if !s.resolvers[server].cooldown.After(time.Now()) {
			healthy = append(healthy, server)
		}
	}
	if len(healthy) == 0 {
		return healthy
	}
	start := int(s.cursor % uint64(len(healthy)))
	return append(append([]string{}, healthy[start:]...), healthy[:start]...)

}

func (s *EnhancedService) dnsQuery(ctx context.Context, host, server string, kind uint16, depth int) (*dns.Msg, error) {
	if depth > 4 {
		return nil, &CodeError{"IP_DNS_UNAVAILABLE"}
	}
	answer, err := s.dnsExchange(ctx, host, server, kind, "udp")
	if err == nil && answer.Truncated {
		answer, err = s.dnsExchange(ctx, host, server, kind, "tcp")
	}
	if err != nil {
		return nil, err
	}
	for _, rr := range answer.Answer {
		if rr.Header().Rrtype == kind {
			return answer, nil
		}
	}
	for _, rr := range answer.Answer {
		if cname, ok := rr.(*dns.CNAME); ok && !strings.EqualFold(dns.Fqdn(host), cname.Target) {
			return s.dnsQuery(ctx, cname.Target, server, kind, depth+1)
		}
	}
	return answer, nil
}

func (s *EnhancedService) dnsExchange(ctx context.Context, host, server string, kind uint16, network string) (*dns.Msg, error) {
	s.mu.Lock()
	h := s.resolvers[server]
	start := maxTime(time.Now(), h.next)
	h.next = start.Add(150 * time.Millisecond)
	s.resolvers[server] = h
	s.mu.Unlock()
	timer := time.NewTimer(time.Until(start))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	answer, err := upstreamqueue.Do(ctx, s.queues["dns"], func(ctx context.Context) (*dns.Msg, error) {
		req := new(dns.Msg)
		req.SetQuestion(dns.Fqdn(host), kind)
		client := &dns.Client{Net: network, Timeout: 2 * time.Second}
		answer, _, err := client.ExchangeContext(ctx, req, server)
		if err != nil || answer == nil || (answer.Rcode != dns.RcodeSuccess && answer.Rcode != dns.RcodeNameError) {
			return nil, &CodeError{"IP_DNS_UNAVAILABLE"}
		}
		return answer, nil
	})
	s.mu.Lock()
	h = s.resolvers[server]
	if err != nil {
		h.failures++
		if h.failures >= 2 {
			h.cooldown = time.Now().Add(30 * time.Second)
		}
	} else {
		h.failures = 0
		h.cooldown = time.Time{}
	}
	s.resolvers[server] = h
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return answer, nil
}

func maxTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}
