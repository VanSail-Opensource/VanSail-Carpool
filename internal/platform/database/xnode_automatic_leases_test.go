package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestAutomaticLeasesReleaseExactBansAndPreserveRangesAndFinancialGrants(t *testing.T) {
	s, owner, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	events := []wire.FirewallEvent{
		{ID: "auto-00000000000000000001", Kind: "detected", IP: "8.8.8.8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now},
		{ID: "auto-00000000000000000002", Kind: "detected", IP: "2001:4860::19", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now},
		{ID: "auto-00000000000000000003", Kind: "detected", IP: "9.9.9.9", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now},
	}
	if err := s.RecordFirewallEvents(ctx, "node", "auto", events, wire.FirewallPolicy{Enabled: true, Protect: true}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdjustBalance(ctx, owner.UserID, 1000, "seed-auto", "", now); err != nil {
		t.Fatal(err)
	}
	paidID := seedHistoricalFirewallGrant(t, s, owner.UserID, "paid-auto", now)
	if _, err := s.GrantAdminFirewallExemption(ctx, owner.UserID, xnode.AdminFirewallExemption{Prefix: "8.8.8.0/24", Hours: 24, Key: "manual-auto"}, now); err != nil {
		t.Fatal(err)
	}
	items := []xnode.AutomaticLeaseGrant{{IP: "8.8.8.8", Generation: 1, Until: now.Add(time.Hour)}, {IP: "2001:4860::19", Generation: 2, Until: now.Add(time.Hour)}, {IP: "9.9.9.9", Generation: 3, Until: now.Add(-time.Nanosecond)}}
	for range 2 {
		if err := s.ReleaseAutomaticLeaseBans(ctx, items, now); err != nil {
			t.Fatal(err)
		}
	}
	bans, err := s.FirewallBans(ctx, 100)
	if err != nil || len(bans) != 2 {
		t.Fatal("unban released a range or used an expired lease", bans, err)
	}
	for _, ban := range bans {
		if ban.Prefix != "2001:4860::/64" && ban.Prefix != "9.9.9.9/32" {
			t.Fatal("unexpected remaining ban", ban)
		}
	}
	var grants int
	var price int64
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM xnode_firewall_exemptions WHERE released_version=0`).Scan(&grants); err != nil || grants != 2 {
		t.Fatal("automatic lease was persisted or existing grant removed", grants, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT price_minor FROM xnode_firewall_exemptions WHERE id=$1`, paidID).Scan(&price); err != nil || price != 125 {
		t.Fatal("paid exemption fact changed", price, err)
	}
	var version, after uint64
	if err := s.db.QueryRowContext(ctx, `SELECT version FROM xnode_firewall_clock WHERE singleton=1`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := s.ReleaseAutomaticLeaseBans(ctx, items, now); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT version FROM xnode_firewall_clock WHERE singleton=1`).Scan(&after); err != nil || after != version {
		t.Fatal("no-op replay changed the durable firewall clock", version, after, err)
	}
}
