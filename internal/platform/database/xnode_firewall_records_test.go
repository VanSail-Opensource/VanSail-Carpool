package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallRecordsMergeReportsBansAndBindCursor(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	if err := store.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	policy := wire.FirewallPolicy{Enabled: true, Protect: true}
	events := make([]wire.FirewallEvent, 0, 102)
	for index := 0; index < 102; index++ {
		ip := fmt.Sprintf("8.8.0.%d", index+1)
		count := uint64(1)
		if index == 0 {
			count = 3
		}
		at := now.Add(-time.Duration(index) * time.Second)
		events = append(events, wire.FirewallEvent{ID: fmt.Sprintf("records-%020d", index+1), Kind: "detected", IP: ip, Rule: "xray_tcp_probe", Count: count, FirstSeen: at, Time: at})
	}
	for index, item := range []struct {
		ip    string
		count uint64
	}{
		{"2001:4860:1234:5678::1", 1}, {"2001:4860:1234:5678::2", 3}, {"2001:4860:1234:5679::1", 1},
	} {
		events = append(events, wire.FirewallEvent{ID: fmt.Sprintf("records-%020d", index+103), Kind: "detected", IP: item.ip, Rule: "xray_tcp_probe", Count: item.count, FirstSeen: now, Time: now})
	}
	if err := store.RecordFirewallEvents(ctx, "node", "records", events, policy, now); err != nil {
		t.Fatal(err)
	}
	filter := xnode.FirewallFilter{Op: "and"}
	request := xnode.FirewallRecordSearch{Filter: filter, Limit: 20}
	seen, bans := map[string]bool{}, 0
	for {
		page, err := store.SearchFirewallRecords(ctx, request, now)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatal("duplicate page item", item.ID)
			}
			seen[item.ID] = true
			if item.Banned {
				bans++
			} else if item.Generation != "" || item.Until != nil {
				t.Fatal("report has a ban action", item)
			}
		}
		if page.NextCursor == "" {
			break
		}
		if request.Cursor == "" {
			changed := request
			changed.Cursor = page.NextCursor
			changed.Filter.Children = []xnode.FirewallFilter{{Field: "status", Value: "reported"}}
			if _, err := store.SearchFirewallRecords(ctx, changed, now); err == nil {
				t.Fatal("cursor accepted a different filter")
			}
		}
		request.Cursor = page.NextCursor
	}
	if len(seen) != 104 || bans != 2 {
		t.Fatalf("merged records = %d, bans = %d", len(seen), bans)
	}
	request.Cursor = ""
	request.Filter.Children = []xnode.FirewallFilter{{Field: "ip", Value: "8.8.0.102"}, {Field: "status", Value: "reported"}}
	page, err := store.SearchFirewallRecords(ctx, request, now)
	if err != nil || len(page.Items) != 1 || page.Items[0].IP != "8.8.0.102" {
		t.Fatal("late report search", page, err)
	}
	request.Filter.Children = []xnode.FirewallFilter{{Field: "ip", Value: "2001:4860:1234:5679::2"}, {Field: "status", Value: "reported"}}
	page, err = store.SearchFirewallRecords(ctx, request, now)
	if err != nil || len(page.Items) != 0 {
		t.Fatal("IPv6 report incorrectly matched another host", page, err)
	}
}
