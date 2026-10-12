package iplookup

import (
	"context"
	"errors"
	"testing"
	"time"
)

type orderedInventoryProvider struct {
	t      *testing.T
	r      *Report
	calls  int
	refuse bool
}

func (p *orderedInventoryProvider) ID() string { return "ipapi" }
func (p *orderedInventoryProvider) Lookup(context.Context, string, ProviderConfig) (ProviderResult, error) {
	p.calls++
	if p.r.Proxy == nil || p.r.Subnet == nil || len(p.r.Subnet.Addresses) != 256 || !p.r.Checkpoint.Enhanced.InventoryDone {
		p.t.Fatal("provider ran before built-in inventory coverage")
	}
	return ProviderResult{ID: "ipapi", Status: "success", Complete: true, Signals: Signals{Proxy: boolean(p.refuse)}}, nil
}

func TestInventoryPrecedesProvidersAndSurvivesRefusals(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		nativeRefusal, providerRefusal bool
	}{
		{"passed", false, false},
		{"inventory refusal", true, false},
		{"later provider refusal", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := DefaultConfig()
			for i := range c.Providers {
				c.Providers[i].Enabled = c.Providers[i].ID == "ipapi"
			}
			s := NewEnhancedService(nil, nil, nil)
			entries := []string{"8.8.8.7/32"}
			if tc.nativeRefusal {
				entries = append(entries, "8.8.8.8/32")
			}
			s.pools[proxyPoolKey(c)] = &proxyPool{indexes: []*prefixIndex{testPrefixIndex(entries...)}, sources: []ProxySourceResult{{ID: "native", Name: "Native"}}, complete: true, expires: time.Now().Add(time.Hour)}
			r := NewReport("inventory-order", "8.8.8.8", c)
			provider := &orderedInventoryProvider{t: t, r: &r, refuse: tc.providerRefusal}
			w := NewWorker(nil, []Provider{provider})
			w.Enhanced = s
			if err := w.runQueuedStages(context.Background(), &r, c, func(Report) error { return nil }); err != nil {
				t.Fatal(err)
			}
			if (provider.calls == 0) != tc.nativeRefusal || r.Subnet.Addresses[7].Status != "flagged" {
				t.Fatal("inventory scan gated or provider short-circuit lost")
			}
			if tc.nativeRefusal || tc.providerRefusal {
				// No live service or IPAPI adapter is configured: external work would fail.
				if err := s.checkSubnet(context.Background(), &r, c, func() error { return nil }); err != nil {
					t.Fatal(err)
				}
				if r.Subnet.Status != "success" || r.Subnet.Reason != "ip_not_passed" || r.Subnet.Addresses[6].Status != "clean" {
					t.Fatalf("built-in scan not retained: %+v", r.Subnet)
				}
				for _, row := range r.Subnet.Addresses {
					if row.Abuse != "skipped" || row.IPAPI != "skipped" {
						t.Fatalf("external checks not skipped: %+v", row)
					}
				}
			} else if r.Subnet.Addresses[6].IPAPI != "pending" {
				t.Fatal("passing requested IP lost its external scan")
			}
			failure := errors.New("must not replay inventory")
			if err := w.runInventory(context.Background(), &r, c, func(Report) error { return failure }); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInventoryProgressOrderAndIncompleteCoverage(t *testing.T) {
	r := NewReport("inventory-partial", "8.8.8.8", DefaultConfig())
	r.Checkpoint.Enhanced = &EnhancedCheckpoint{Phase: "checking"}
	pool := &proxyPool{complete: false, indexes: []*prefixIndex{testPrefixIndex("8.8.8.7/32")}, sources: []ProxySourceResult{{ID: "abuse", Category: AbuseCategory}}}
	if err := initializeSubnet(context.Background(), &r, pool); err != nil {
		t.Fatal(err)
	}
	finishInventorySubnet(&r, "proxy_incomplete")
	if r.Subnet.Status != "partial" || r.Subnet.Addresses[6].Status != "unknown" || r.Subnet.Addresses[7].Abuse != "flagged" {
		t.Fatal("partial coverage or abuse-list evidence lost")
	}
	p := CheckProgress(r)
	if p.Tasks[0].Key != "proxy" || p.Tasks[1].Key != "subnet" {
		t.Fatal("inventory tasks do not lead progress")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := initializeSubnet(ctx, &r, pool); !errors.Is(err, context.Canceled) {
		t.Fatal("scan ignored cancellation")
	}
}
