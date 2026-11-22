package database

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallRepeatedParticipantExpiryAvoidsWritesAndRetainsFinalRelease(t *testing.T) {
	for _, source := range []bool{false, true} {
		t.Run(fmt.Sprint("source=", source), func(t *testing.T) {
			s, _, _ := xnodeTrafficFixture(t)
			ctx, now := context.Background(), time.Now().UTC()
			if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "second", Name: "Second", ConfigID: "config", SettingsCipher: "encrypted", Enabled: true, MultiplierFP: 1000000}); err != nil {
				t.Fatal(err)
			}
			for _, node := range []string{"node", "second"} {
				if err := s.ReconcileFirewallMember(ctx, node, true, true, 0); err != nil {
					t.Fatal(err)
				}
			}
			policy := wire.FirewallPolicy{Enabled: true, Protect: true}
			detected := wire.FirewallEvent{ID: fmt.Sprintf("noop-%020d", 1), Kind: "detected", IP: "8.8.8.8", Rule: "xray_tcp_probe", Count: 3, FirstSeen: now, Time: now}
			if err := s.RecordFirewallEvents(ctx, "node", "noop", []wire.FirewallEvent{detected}, policy, now); err != nil {
				t.Fatal(err)
			}
			bans, err := s.FirewallBans(ctx, 10)
			if err != nil || len(bans) != 1 || bans[0].PendingNodes != 2 {
				t.Fatal("fixture missing two-participant generation", bans, err)
			}
			table := "xnode_firewall_bans"
			expiry := wire.FirewallEvent{Kind: "expired", Prefix: bans[0].Prefix, Generation: bans[0].Generation, Time: now}
			if source {
				table, expiry.Kind, expiry.Rule = "xnode_firewall_sources", "source_expired", "source"
				if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_firewall_sources(id,generation,until_at,refresh_seconds,issued_version,participants,expired)
 SELECT 'source',generation,until_at,refresh_seconds,issued_version,participants,expired FROM xnode_firewall_bans WHERE generation=$1`, expiry.Generation); err != nil {
					t.Fatal(err)
				}
			}
			type snapshot struct {
				xmin, releasedAt string
				released         uint64
				expired          []byte
			}
			read := func() snapshot {
				t.Helper()
				var v snapshot
				if err := s.db.QueryRowContext(ctx, `SELECT xmin::text,released_version,COALESCE(released_at,''),expired FROM `+table+` WHERE generation=$1`, expiry.Generation).Scan(&v.xmin, &v.released, &v.releasedAt, &v.expired); err != nil {
					t.Fatal(err)
				}
				return v
			}
			report := func(node, namespace string, sequence int) {
				t.Helper()
				expiry.ID = fmt.Sprintf("%s-%020d", namespace, sequence)
				if err := s.RecordFirewallEvents(ctx, node, namespace, []wire.FirewallEvent{expiry}, policy, now); err != nil {
					t.Fatal(err)
				}
			}
			report("node", "noop", 2)
			partial := read()
			bitmap, err := firewallBitmap(partial.expired)
			if err != nil || bitmap.GetCardinality() != 1 || partial.released != 0 {
				t.Fatal("first participant prematurely released the generation", partial, err)
			}
			// This is a NEW journal sequence, not the existing cursor replay path.
			report("node", "noop", 3)
			repeated := read()
			if repeated.xmin != partial.xmin || repeated.released != 0 || !bytes.Equal(repeated.expired, partial.expired) {
				t.Fatal("duplicate partial expiry rewrote the durable generation")
			}
			var cursor uint64
			if err := s.db.QueryRowContext(ctx, `SELECT sequence FROM xnode_firewall_cursors WHERE node_id='node' AND namespace='noop'`).Scan(&cursor); err != nil || cursor != 3 {
				t.Fatal("no-op expiry failed to commit journal progress", cursor, err)
			}
			report("second", "other", 1)
			released := read()
			bitmap, err = firewallBitmap(released.expired)
			if err != nil || bitmap.GetCardinality() != 2 || released.released == 0 || released.releasedAt == "" || released.xmin == partial.xmin {
				t.Fatal("last participant did not persist final release", released, err)
			}
			if source {
				var version uint64
				if err := s.db.QueryRowContext(ctx, `SELECT source_version FROM xnode_firewall_clock WHERE singleton=1`).Scan(&version); err != nil || version != released.released {
					t.Fatal("source final release lost its version", version, err)
				}
			}
			report("second", "other", 2)
			if repeated = read(); repeated.xmin != released.xmin || repeated.released != released.released || repeated.releasedAt != released.releasedAt {
				t.Fatal("repeated final expiry changed the released generation")
			}
		})
	}
}
