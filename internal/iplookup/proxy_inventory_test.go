package iplookup

import (
	"context"
	"github.com/miekg/dns"
	"testing"
	"time"
)

func TestPoolIgnoresEndpointDNSFailuresAndCachesForSixHours(t *testing.T) {
	for _, complete := range []bool{true, false} {
		t.Run(map[bool]string{true: "complete", false: "partial"}[complete], func(t *testing.T) {
			s := diagnosticService(t)
			code := dns.RcodeSuccess
			if !complete {
				code = dns.RcodeRefused
			}
			server, _ := diagnosticDNS(t, code)
			calls := 0
			s.inventoryFetch = func(context.Context, string, Config) ([]byte, error) {
				calls++
				return []byte("edge.example.net:443"), nil
			}
			c := DefaultConfig()
			c.ProxySources = []ProxySource{{ID: "one", Name: "One", URL: "https://example.net/list?token=secret", Enabled: true}}
			c.DNSResolvers = []string{server}
			p, err := s.pool(context.Background(), c)
			if err != nil {
				t.Fatal(err)
			}
			if !p.complete || time.Until(p.expires) < 6*time.Hour-time.Minute {
				t.Fatalf("pool: %+v", p)
			}
			if !complete && (p.sources[0].Skipped != 1 || p.sources[0].DNSFailed != 1 || p.sources[0].ErrorCode != "") {
				t.Fatal("DNS failures were not ignored as skipped endpoints")
			}
			if p.sources[0].URL != "https://example.net/list" {
				t.Fatal("query secret exposed")
			}
			if _, err := s.pool(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			want := 1
			if calls != want {
				t.Fatalf("downloads=%d want=%d", calls, want)
			}
		})
	}
}

func TestInventoryNoAddressIsSkippedWithoutPartialCoverage(t *testing.T) {
	s := diagnosticService(t)
	server, _ := diagnosticDNS(t, dns.RcodeNameError)
	s.inventoryFetch = func(context.Context, string, Config) ([]byte, error) {
		return []byte("absent.example.net:443\n8.8.8.8:443"), nil
	}
	c := DefaultConfig()
	c.DNSResolvers = []string{server}
	ips, result := s.collectSource(context.Background(), ProxySource{ID: "one", URL: "https://example.net/list"}, c, true, nil)
	if result.Status != "success" || result.Skipped != 1 || result.Processed != 2 || len(ips) != 1 {
		t.Fatalf("result %+v", result)
	}
}
