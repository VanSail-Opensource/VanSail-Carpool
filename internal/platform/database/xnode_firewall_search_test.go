package database

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallSearchScansBeyondFirstPageAndMatchesIPv6Overlap(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := store.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	policy := wire.FirewallPolicy{Enabled: true, Protect: true}
	for batch := 0; batch < 3; batch++ {
		events := []wire.FirewallEvent{}
		for offset := 0; offset < 100; offset++ {
			index := batch*100 + offset
			ip := fmt.Sprintf("8.8.%d.%d", index/256, index%256)
			at := now.Add(-time.Duration(300-index) * time.Second)
			events = append(events, wire.FirewallEvent{ID: fmt.Sprintf("search-%020d", index+1), Kind: "detected", IP: ip, Rule: "xray_tcp_probe", Count: 3, FirstSeen: at, Time: at})
		}
		if err := store.RecordFirewallEvents(ctx, "node", "search", events, policy, now); err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.SearchFirewallBans(ctx, netip.MustParsePrefix("8.8.0.0/32"), "", 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].Prefix != "8.8.0.0/32" {
		t.Fatalf("oldest match beyond 200: %#v, %v", page, err)
	}
	all := map[string]bool{}
	cursor := ""
	for {
		page, err = store.SearchFirewallBans(ctx, netip.MustParsePrefix("8.8.0.255/24").Masked(), cursor, 17)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if all[item.Generation] {
				t.Fatal("duplicate cursor item")
			}
			all[item.Generation] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(all) != 256 {
		t.Fatal("CIDR page loss", len(all))
	}
	event := wire.FirewallEvent{ID: "ipv6-search-00000000000000000001", Kind: "detected", IP: "2001:4860:1234:5678::8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now}
	if err := store.RecordFirewallEvents(ctx, "node", "ipv6-search", []wire.FirewallEvent{event}, policy, now); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"2001:4860:1234:5678::abcd/128", "2001:4860:1234::/48"} {
		page, err = store.SearchFirewallBans(ctx, netip.MustParsePrefix(query), "", 10)
		if err != nil || len(page.Items) != 1 {
			t.Fatal("IPv6 overlap", query, page, err)
		}
	}
}
