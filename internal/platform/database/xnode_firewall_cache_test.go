package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallPagesFollowCommittedVersionsAndRemainPrivate(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	key := firewallPageKey{target: 1, full: true, limit: 256}
	page, _, err := s.FirewallPage(ctx, key.after, key.target, key.full, key.cursor, key.limit)
	if err != nil || len(page.Bans) != 0 {
		t.Fatal(page, err)
	}
	event := wire.FirewallEvent{ID: fmt.Sprintf("cached-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now}
	if err := s.RecordFirewallEvents(ctx, "node", "cached", []wire.FirewallEvent{event}, wire.FirewallPolicy{Enabled: true, Protect: true}, now); err != nil {
		t.Fatal(err)
	}
	var version uint64
	if err := s.db.QueryRowContext(ctx, `SELECT version FROM xnode_firewall_clock WHERE singleton=1`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	page, _, err = s.FirewallPage(ctx, 0, version, true, "", 256)
	if err != nil || len(page.Bans) != 1 {
		t.Fatal("committed detection missing", page, err)
	}
	page.Bans[0].Prefix = "modified"
	again, _, err := s.FirewallPage(ctx, 0, version, true, "", 256)
	if err != nil || len(again.Bans) != 1 || again.Bans[0].Prefix != "8.8.8.8/32" {
		t.Fatal("caller changed another node's projection", again, err)
	}
}

func TestFirewallPageCacheBoundsExpiryAndStaleReaders(t *testing.T) {
	c := &firewallPageCache{}
	ctx, now := context.Background(), time.Now()
	loads := 0
	load := func() (wire.FirewallUpdate, string, error) {
		loads++
		return wire.FirewallUpdate{Version: 2}, "next", nil
	}
	key := firewallPageKey{target: 2, limit: 256}
	for range 2 {
		if _, _, err := c.read(ctx, 2, key, load); err != nil {
			t.Fatal(err)
		}
	}
	if loads != 1 {
		t.Fatal("unchanged snapshot reread", loads)
	}
	c.get(3, key, now)
	c.put(2, key, wire.FirewallUpdate{}, "")
	if c.lru.Len() != 0 || c.generation != 3 {
		t.Fatal("older in-flight reader republished stale generation")
	}
	for i := range firewallPageCacheEntries + 10 {
		key.cursor = fmt.Sprint(i)
		c.put(3, key, wire.FirewallUpdate{}, "")
	}
	if c.lru.Len() != firewallPageCacheEntries || c.bytes > firewallPageCacheBytes {
		t.Fatal("unbounded page cache", c.lru.Len(), c.bytes)
	}
	if _, _, ok := c.get(3, key, now.Add(2*firewallPageCacheAge)); ok {
		t.Fatal("expired snapshot reused")
	}
	key.cursor = "failure"
	failure := errors.New("database unavailable")
	for range 2 {
		_, _, err := c.read(ctx, 3, key, func() (wire.FirewallUpdate, string, error) { return wire.FirewallUpdate{}, "", failure })
		if !errors.Is(err, failure) {
			t.Fatal("error cached as an empty snapshot", err)
		}
	}
}
