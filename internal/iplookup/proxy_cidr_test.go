package iplookup

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

const proxyCIDRFixture = `102.129.155.0/24
102.129.156.0/24
2001:ac8:16::/48
2001:ac8:17::/48
2001:ac8:20::/48
2001:ac8:21::/48
2001:ac8:22::/48
2001:ac8:23::/48
2001:ac8:24::/48
2001:ac8:25::/48
2001:ac8:26::/48
2001:ac8:27::/48
2001:ac8:28::/48
2001:ac8:29::/48
2001:ac8:2::/48`

func TestCIDRSourceMatchesNetworksWithoutExpansionOrDNS(t *testing.T) {
	s := NewEnhancedService(nil, nil, nil)
	s.inventoryFetch = func(context.Context, string, Config) ([]byte, error) {
		return []byte(proxyCIDRFixture + "\n102.129.155.17:443\n102.129.155.16/28"), nil
	}
	c := DefaultConfig()
	c.ProxySources = []ProxySource{{ID: "ranges", Name: "Ranges", URL: "https://example.net/ranges", Enabled: true}}
	p, err := s.pool(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if !p.complete || p.indexes[0].count() != 17 || len(s.dns) != 0 || p.sources[0].Count != 17 || p.sources[0].Processed != 17 || p.sources[0].Skipped != 0 {
		t.Fatalf("CIDRs expanded, skipped or sent to DNS: %+v", p.sources)
	}
	cases := []struct {
		ip   string
		want []string
	}{
		{"102.129.155.0", []string{"ranges"}},
		{"102.129.155.17", []string{"ranges"}},
		{"102.129.155.255", []string{"ranges"}},
		{"102.129.156.128", []string{"ranges"}},
		{"102.129.154.255", []string{}},
		{"102.129.157.0", []string{}},
		{"2001:ac8:16::", []string{"ranges"}},
		{"2001:ac8:16:ffff:ffff:ffff:ffff:ffff", []string{"ranges"}},
		{"2001:ac8:17::1234", []string{"ranges"}},
		{"2001:ac8:2::1", []string{"ranges"}},
		{"2001:ac8:19::1", []string{}},
		{"2001:ac8:160::1", []string{}},
		{"::ffff:102.129.155.17", []string{"ranges"}},
	}
	for _, tc := range cases {
		t.Run(tc.ip, func(t *testing.T) {
			if got := p.assessment(tc.ip).Matches; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	for _, prefix := range strings.Fields(proxyCIDRFixture) {
		ip := strings.SplitN(prefix, "/", 2)[0]
		if got := p.assessment(ip).Matches; !reflect.DeepEqual(got, []string{"ranges"}) {
			t.Fatalf("missing supplied prefix %s: %v", prefix, got)
		}
	}
}

func TestCachedCIDRSourcesRetainDistinctAttributionAfterFailure(t *testing.T) {
	s := diagnosticService(t)
	c := DefaultConfig()
	c.ProxySources = []ProxySource{{ID: "one", Name: "One", URL: "https://example.net/one", Enabled: true}, {ID: "two", Name: "Two", URL: "https://example.net/two", Enabled: true}}
	settings := &sourceSettings{}
	settings.set(t, c)
	s.settings = settings
	var failed atomic.Bool
	s.inventoryFetch = func(context.Context, string, Config) ([]byte, error) {
		if failed.Load() {
			return nil, errors.New("download unavailable")
		}
		return []byte("102.129.155.0/24\n2001:ac8:16::/48"), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.StartSourceTests(ctx)
	t.Cleanup(func() { cancel(); s.WaitSourceTests() })
	awaitSource(t, s, "one")
	awaitSource(t, s, "two")
	status, err := s.SourcesStatus(ctx)
	if err != nil || status.UniqueIPs != 2 || status.Sources[0].Count != 2 {
		t.Fatalf("prefix diagnostics: %+v err=%v", status, err)
	}
	verify := func(complete bool) {
		t.Helper()
		p, err := s.pool(ctx, c)
		if err != nil || p.complete != complete {
			t.Fatalf("prefix freshness: %+v err=%v", p, err)
		}
		for _, ip := range []string{"102.129.155.27", "2001:ac8:16::1234"} {
			if got := p.assessment(ip).Matches; !reflect.DeepEqual(got, []string{"one", "two"}) {
				t.Fatalf("lost source attribution for %s: %v", ip, got)
			}
		}
	}
	verify(true)
	failed.Store(true)
	if _, err := s.TestSource(ctx, "one"); err != nil {
		t.Fatal(err)
	}
	awaitSource(t, s, "one")
	verify(false)
}
