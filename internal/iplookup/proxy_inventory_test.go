package iplookup

import (
	"context"
	"github.com/miekg/dns"
	"net/netip"
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

func TestPartialInventoryPrioritizesPublishedEvidenceAtLimit(t *testing.T) {
	previous := map[netip.Prefix]bool{netip.MustParsePrefix("8.8.8.8/32"): true, netip.MustParsePrefix("9.9.9.9/32"): true}
	incoming := map[netip.Prefix]bool{netip.MustParsePrefix("1.1.1.1/32"): true, netip.MustParsePrefix("8.8.4.4/32"): true, netip.MustParsePrefix("1.0.0.1/32"): true}
	retained := retainSourcePrefixes(previous, incoming, 3)
	if len(retained) != 3 || !retained[netip.MustParsePrefix("8.8.8.8/32")] || !retained[netip.MustParsePrefix("9.9.9.9/32")] {
		t.Fatal("partial refresh displaced published positives at the limit")
	}
	if len(previous) != 2 || len(incoming) != 3 {
		t.Fatal("retention mutated a published or collected map")
	}
}
