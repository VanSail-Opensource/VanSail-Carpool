package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallSweepSharesAudienceWithoutExpiringActiveMembers(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "other", Name: "Other node", ConfigID: "config", SettingsCipher: "encrypted", Enabled: true, MultiplierFP: 1000000}); err != nil {
		t.Fatal(err)
	}
	for _, node := range []string{"node", "other"} {
		if err := s.ReconcileFirewallMember(ctx, node, true, true, 0); err != nil {
			t.Fatal(err)
		}
	}
	for i, ip := range []string{"8.8.8.8", "9.9.9.9", "1.1.1.1"} {
		event := wire.FirewallEvent{ID: fmt.Sprintf("sweep-%020d", i+1), Kind: "detected", IP: ip, Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now}
		if err := s.RecordFirewallEvents(ctx, "node", "sweep", []wire.FirewallEvent{event}, wire.FirewallPolicy{Enabled: true, Protect: true}, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ReconcileFirewallMember(ctx, "node", false, false, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SweepFirewall(ctx, now); err != nil {
		t.Fatal(err)
	}
	bans, err := s.FirewallBans(ctx, 10)
	if err != nil || len(bans) != 3 {
		t.Fatal("sweep lost active-node bans", bans, err)
	}
	for _, ban := range bans {
		if ban.PendingNodes != 1 {
			t.Fatal("shared bitmap mutated across bans", ban)
		}
	}
	if err = s.ReconcileFirewallMember(ctx, "other", false, false, 0); err != nil {
		t.Fatal(err)
	}
	if err = s.SweepFirewall(ctx, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	bans, err = s.FirewallBans(ctx, 10)
	if err != nil || len(bans) != 0 {
		t.Fatal("retirement waited for history-pruning interval", bans, err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE xnode_firewall_bans SET released_at=? WHERE released_version>0`, stamp(now.Add(-8*24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		after time.Duration
		want  int
	}{{30 * time.Second, 3}, {time.Minute, 0}} {
		if err = s.SweepFirewall(ctx, now.Add(step.after)); err != nil {
			t.Fatal(err)
		}
		var count int
		if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_firewall_bans`).Scan(&count); err != nil || count != step.want {
			t.Fatal("history-pruning cadence", count, step, err)
		}
	}
}
