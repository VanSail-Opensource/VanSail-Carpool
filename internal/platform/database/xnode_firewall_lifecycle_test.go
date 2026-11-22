package database

import (
	"context"
	"testing"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestFirewallSweepRepairsPermanentRemovalWithoutRetiringOfflineOrDisabledMembers(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	nodes := []string{"node", "retired", "absent", "offline", "disabled", "removing"}
	ordinals := map[string]uint32{}
	for _, node := range nodes {
		if node != "node" {
			if err := s.SaveXNodeNode(ctx, xnode.Node{ID: node, Name: node, ConfigID: "config", SettingsCipher: "encrypted", Enabled: true, MultiplierFP: xnode.MultiplierScale}); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.ReconcileFirewallMember(ctx, node, true, true, 0); err != nil {
			t.Fatal(err)
		}
		var ordinal uint32
		if err := s.db.QueryRowContext(ctx, `SELECT ordinal FROM xnode_firewall_members WHERE node_id=$1`, node).Scan(&ordinal); err != nil {
			t.Fatal(err)
		}
		ordinals[node] = ordinal
	}
	bitmap := func(members ...string) []byte {
		t.Helper()
		b := roaring.New()
		for _, member := range members {
			b.Add(ordinals[member])
		}
		raw, err := b.ToBytes()
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	var before uint64
	if err := s.db.QueryRowContext(ctx, `SELECT version FROM xnode_firewall_clock WHERE singleton=1`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, ban := range []struct {
		generation, ip string
		members        []string
	}{
		{"old-ban", "8.8.8.8", []string{"node", "retired", "absent"}},
		{"held-ban", "9.9.9.9", []string{"node", "offline", "disabled", "removing"}},
	} {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_firewall_bans(generation,prefix,ip,rule,detected_count,detected_at,until_at,refresh_seconds,issued_version,participants,expired)
 VALUES($1,$2,$3,'xray_tcp_probe','3',$4,$5,60,$6,$7,$8)`, ban.generation, ban.ip+"/32", ban.ip, stamp(now.Add(-2*time.Hour)), stamp(now.Add(-time.Hour)), before, bitmap(ban.members...), bitmap("node")); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{
		`INSERT INTO xnode_firewall_sources(id,generation,until_at,refresh_seconds,issued_version,participants,expired)
 SELECT 'legacy-source',generation,until_at,refresh_seconds,issued_version,participants,expired FROM xnode_firewall_bans WHERE generation='old-ban'`,
		`UPDATE xnode_firewall_members SET applied=(SELECT version FROM xnode_firewall_clock),source_applied=(SELECT source_version FROM xnode_firewall_clock),force_sync=0`,
		`UPDATE xnode_nodes SET enabled=0,retired_at=updated_at WHERE id='retired'`,
		`DELETE FROM xnode_nodes WHERE id='absent'`,
		`UPDATE xnode_firewall_members SET active=1 WHERE node_id='absent'`, // Legacy orphan, predating the deletion trigger.
		`UPDATE xnode_nodes SET enabled=0 WHERE id='disabled'`,
		`UPDATE xnode_nodes SET enabled=0,removing_at=updated_at WHERE id='removing'`,
		`DELETE FROM outbox_jobs WHERE kind='xnode_firewall'`,
	} {
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	// A repair must revisit earlier generations, even halfway through a sweep.
	s.firewallSweepCursor = "s:already-visited"
	if err := s.SweepFirewall(ctx, now); err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		var active, force int
		var ordinal uint32
		var epoch, applied uint64
		if err := s.db.QueryRowContext(ctx, `SELECT ordinal,active,force_sync,connection_epoch,applied FROM xnode_firewall_members WHERE node_id=$1`, node).Scan(&ordinal, &active, &force, &epoch, &applied); err != nil {
			t.Fatal(err)
		}
		want := 1
		if node == "retired" || node == "absent" {
			want = 0
		}
		if ordinal != ordinals[node] || active != want || force != 0 || epoch != 1 || applied != before {
			t.Fatal("lifecycle repair changed an unrelated member fact", node, ordinal, active, force, epoch, applied)
		}
	}
	var version, sourceVersion, banRelease, sourceRelease uint64
	if err := s.db.QueryRowContext(ctx, `SELECT c.version,c.source_version,b.released_version,s.released_version
 FROM xnode_firewall_clock c CROSS JOIN xnode_firewall_bans b CROSS JOIN xnode_firewall_sources s
 WHERE b.generation='old-ban' AND s.id='legacy-source' AND s.generation='old-ban'`).Scan(&version, &sourceVersion, &banRelease, &sourceRelease); err != nil {
		t.Fatal(err)
	}
	if version != before+2 || banRelease != before+1 || sourceRelease != version || sourceVersion != version {
		t.Fatal("final ban/source release clocks were not exact", before, version, sourceVersion, banRelease, sourceRelease)
	}
	remaining, err := s.FirewallBans(ctx, 10)
	if err != nil || len(remaining) != 1 || remaining[0].Generation != "held-ban" || remaining[0].PendingNodes != 3 {
		t.Fatal("offline, disabled or removing participants were released without confirmation", remaining, err)
	}
	var jobs, removedJobs int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE payload::jsonb->>'nodeId' IN ('retired','absent')) FROM outbox_jobs WHERE kind='xnode_firewall'`).Scan(&jobs, &removedJobs); err != nil || jobs != 8 || removedJobs != 0 {
		t.Fatal("release delivery was lost or queued for a removed node", jobs, removedJobs, err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE kind='xnode_firewall'`); err != nil {
		t.Fatal(err)
	}
	for _, node := range []string{"retired", "absent", "never-existed"} {
		if err := s.ReconcileFirewallMember(ctx, node, true, true, version+100); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SweepFirewall(ctx, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var after uint64
	var activeRemoved, retainedRemoved int
	if err := s.db.QueryRowContext(ctx, `SELECT version FROM xnode_firewall_clock WHERE singleton=1`).Scan(&after); err != nil || after != version {
		t.Fatal("stale reactivation or unchanged sweep advanced the clock", after, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER(WHERE active=1),COUNT(*) FROM xnode_firewall_members WHERE node_id IN ('retired','absent','never-existed')`).Scan(&activeRemoved, &retainedRemoved); err != nil || activeRemoved != 0 || retainedRemoved != 2 {
		t.Fatal("removed membership was resurrected or recreated", activeRemoved, retainedRemoved, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_firewall'`).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatal("unchanged or stale lifecycle requests requeued fleet delivery", jobs, err)
	}
}
