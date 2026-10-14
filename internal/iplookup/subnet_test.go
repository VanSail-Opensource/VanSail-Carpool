package iplookup

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type subnetProvider struct {
	calls     []string
	blockDone *bool
}

func (p *subnetProvider) ID() string { return "ipapi" }
func (p *subnetProvider) Lookup(_ context.Context, ip string, _ ProviderConfig) (ProviderResult, error) {
	if !*p.blockDone {
		return ProviderResult{}, &CodeError{"IP_LOOKUP_INVALID_RESPONSE"}
	}
	p.calls = append(p.calls, ip)
	flags := Signals{Abuse: boolean(false), Proxy: boolean(ip == "8.8.8.10"), VPN: boolean(false), Tor: boolean(false)}
	return ProviderResult{ID: "ipapi", Status: "success", Signals: flags, Complete: true}, nil
}
func TestSubnetShortCircuitsEachAddressInSourceOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "abuseipdb", Capacity: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = queue.Shutdown(context.Background()) }()
	c := DefaultConfig()
	for i := range c.Providers {
		c.Providers[i].Enabled = c.Providers[i].ID == "abuseipdb" || c.Providers[i].ID == "ipapi"
	}
	live := NewLiveService(providerSettings{value: "key"}, map[string]*upstreamqueue.Queue{"abuseipdb": queue})
	blockDone := false
	live.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("network") != "8.8.8.0/24" || req.URL.Query().Get("maxAgeInDays") != "30" {
			t.Errorf("incorrect block request: %s", req.URL.RawQuery)
		}
		blockDone = true
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"networkAddress":"8.8.8.0","netmask":"255.255.255.0","minAddress":"8.8.8.0","maxAddress":"8.8.8.255","reportedAddress":[{"ipAddress":"8.8.8.9","numReports":2,"abuseConfidenceScore":10}]}}`))}, nil
	})
	s := NewEnhancedService(providerSettings{value: "key"}, nil, live)
	provider := &subnetProvider{blockDone: &blockDone}
	s.ipapi = provider
	s.pools[proxyPoolKey(c)] = &proxyPool{indexes: []*prefixIndex{testPrefixIndex("8.8.8.7/32")}, sources: []ProxySourceResult{{ID: "named-list", Name: "Named list", Category: ProxyPoolCategory}}, complete: true, expires: time.Now().Add(time.Minute)}
	flags := Signals{Abuse: boolean(false), Proxy: boolean(false), VPN: boolean(false), Tor: boolean(false)}
	r := NewReport("subnet", "8.8.8.8", c)
	r.Checkpoint.Enhanced = &EnhancedCheckpoint{IPAPI: &ProviderResult{Signals: flags, Status: "success"}}
	r.LiveDetails = &LiveDetails{IP: r.IP}
	if err := s.checkSubnet(ctx, &r, c, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	for _, ip := range provider.calls {
		if ip == "8.8.8.7" || ip == "8.8.8.9" || ip == r.IP {
			t.Fatalf("unnecessary IPAPI lookup for %s", ip)
		}
	}
	if len(provider.calls) != 253 {
		t.Fatalf("IPAPI calls=%d", len(provider.calls))
	}
	for _, last := range []int{7, 9, 10} {
		if r.Subnet.Addresses[last].Status != "flagged" {
			t.Fatalf("address %d not flagged", last)
		}
	}
	if r.Subnet.Addresses[7].Abuse != "skipped" || r.Subnet.Addresses[9].IPAPI != "skipped" || r.Subnet.Addresses[11].Status != "clean" {
		t.Fatal("per-address precedence violated")
	}
}
func TestSubnetSkipsAbuseItemWhenBlockIsClean(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "abuseipdb", Capacity: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = queue.Shutdown(context.Background()) }()
	c := DefaultConfig()
	for i := range c.Providers {
		c.Providers[i].Enabled = c.Providers[i].ID == "abuseipdb" || c.Providers[i].ID == "ipapi"
	}
	live := NewLiveService(providerSettings{value: "key"}, map[string]*upstreamqueue.Queue{"abuseipdb": queue})
	live.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Errorf("unexpected block request after a clean /24: %s", req.URL.RawQuery)
		return nil, nil
	})
	s := NewEnhancedService(providerSettings{value: "key"}, nil, live)
	done := true
	provider := &subnetProvider{blockDone: &done}
	s.ipapi = provider
	s.pools[proxyPoolKey(c)] = &proxyPool{complete: true, expires: time.Now().Add(time.Minute)}
	zero := 0
	r := NewReport("subnet-clean", "8.8.8.8", c)
	r.Checkpoint.Enhanced = &EnhancedCheckpoint{}
	r.LiveDetails = &LiveDetails{IP: r.IP, Block: AbuseBlock{LiveMeta: LiveMeta{Status: "success", Source: "abuseipdb"}, Prefix: "8.8.8.0/24", ReportedAddresses: &zero}}
	if err := s.checkSubnet(ctx, &r, c, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(provider.calls) != 256 {
		t.Fatalf("IPAPI calls=%d", len(provider.calls))
	}
	flagged := 0
	for _, row := range r.Subnet.Addresses {
		if row.Abuse != "skipped" {
			t.Fatalf("abuse item not skipped for %s: %s", row.IP, row.Abuse)
		}
		if row.IP == "8.8.8.10" {
			flagged++
			if row.Status != "flagged" {
				t.Fatalf("risk row not flagged: %+v", row)
			}
			continue
		}
		if row.Status != "clean" {
			t.Fatalf("row %s not clean: %+v", row.IP, row)
		}
	}
	if flagged != 1 || r.Subnet.Status != "success" {
		t.Fatalf("subnet outcome wrong: flagged=%d status=%s", flagged, r.Subnet.Status)
	}
}
