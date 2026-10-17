package database

import (
	"context"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"net/netip"
	"sync"
	"testing"
	"time"
)

func TestFirewallReplayAndAllNodeExpiry(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := store.SaveXNodeNode(ctx, xnode.Node{ID: "node-two", Name: "Second", ConfigID: "config", SettingsCipher: "encrypted", Enabled: true, MultiplierFP: 1000000}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"node", "node-two"} {
		if err := store.ReconcileFirewallMember(ctx, id, true, true, 0); err != nil {
			t.Fatal(err)
		}
	}
	policy := wire.FirewallPolicy{Enabled: true, Protect: true}
	event := wire.FirewallEvent{ID: fmt.Sprintf("events-one-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now}
	for range 2 {
		if err := store.RecordFirewallEvents(ctx, "node", "events-one", []wire.FirewallEvent{event}, policy, now); err != nil {
			t.Fatal(err)
		}
	}
	bans, err := store.FirewallBans(ctx, 100)
	if err != nil || len(bans) != 1 || bans[0].PendingNodes != 2 {
		t.Fatalf("replayed detection: %+v %v", bans, err)
	}
	generation := bans[0].Generation
	// One node expiring never removes a still participating offline node.
	expiry := wire.FirewallEvent{ID: fmt.Sprintf("events-one-%020d", 2), Kind: "expired", Prefix: "8.8.8.8/32", Generation: generation, Time: now.Add(72 * time.Hour)}
	if err = store.RecordFirewallEvents(ctx, "node", "events-one", []wire.FirewallEvent{expiry}, policy, now); err != nil {
		t.Fatal(err)
	}
	bans, err = store.FirewallBans(ctx, 100)
	if err != nil || len(bans) != 1 || bans[0].PendingNodes != 1 {
		t.Fatalf("early global expiry: %+v %v", bans, err)
	}
	expiry.ID = fmt.Sprintf("events-two-%020d", 1)
	if err = store.RecordFirewallEvents(ctx, "node-two", "events-two", []wire.FirewallEvent{expiry}, policy, now); err != nil {
		t.Fatal(err)
	}
	bans, err = store.FirewallBans(ctx, 100)
	if err != nil || len(bans) != 0 {
		t.Fatalf("global expiry missing: %+v %v", bans, err)
	}
}
func TestFirewallPaidExemptionDebitsOnceAndPreservesEvidence(t *testing.T) {
	store, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := store.AdjustBalance(ctx, identity.UserID, 10000, "seed", "", now); err != nil {
		t.Fatal(err)
	}
	if err := store.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	policy := wire.FirewallPolicy{Enabled: true, Protect: true}
	event := wire.FirewallEvent{ID: fmt.Sprintf("events-paid-%020d", 1), Kind: "detected", IP: "2001:4860:1234:5678::8", Rule: "closed_port", Count: 3, FirstSeen: now, Time: now}
	if err := store.RecordFirewallEvents(ctx, "node", "events-paid", []wire.FirewallEvent{event}, policy, now); err != nil {
		t.Fatal(err)
	}
	input := xnode.FirewallPurchase{IP: event.IP, Key: "same-payment-key", Price: "1.25", Hours: 24}
	var wg sync.WaitGroup
	grants := make(chan xnode.FirewallGrant, 8)
	failures := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			grant, err := store.PurchaseFirewallExemption(ctx, identity.UserID, input, 125, now)
			if err != nil {
				failures <- err
			} else {
				grants <- grant
			}
		}()
	}
	wg.Wait()
	close(grants)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	id := ""
	for grant := range grants {
		if id != "" && id != grant.ID {
			t.Fatal("retry created a second grant")
		}
		id = grant.ID
	}
	balance, err := store.Balance(ctx, identity.UserID)
	if err != nil || balance.Minor != "9875" {
		t.Fatalf("duplicate debit: %+v %v", balance, err)
	}
	member, err := store.FirewallMember(ctx, "node")
	if err != nil {
		t.Fatal(err)
	}
	page, _, err := store.FirewallPage(ctx, 0, member.Version, true, "", 256)
	if err != nil || len(page.Bans) != 1 || len(page.Exemptions) != 1 || page.Bans[0].Prefix != "2001:4860:1234:5678::/64" {
		t.Fatalf("exemption erased evidence: %+v %v", page, err)
	}
	containsEffective := func(raw string) bool {
		t.Helper()
		address := netip.MustParseAddr(raw)
		found := false
		if err := store.FirewallPrefixes(ctx, func(prefixes []string) error {
			for _, prefix := range prefixes {
				if netip.MustParsePrefix(prefix).Contains(address) {
					found = true
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return found
	}
	if containsEffective(event.IP) || !containsEffective("2001:4860:1234:5678::9") {
		t.Fatal("paid exact exemption did not update effective Abuse membership")
	}
	if err = store.SweepFirewall(ctx, now.Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	page, _, err = store.FirewallPage(ctx, 0, member.Version+1, true, "", 256)
	if err != nil || len(page.Exemptions) != 0 || len(page.Bans) != 1 {
		t.Fatalf("expiry did not restore blacklist: %+v %v", page, err)
	}
	if !containsEffective(event.IP) {
		t.Fatal("exemption expiry did not restore effective Abuse membership")
	}
}
