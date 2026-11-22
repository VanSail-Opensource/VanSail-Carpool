package database

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestFirewallLifecycleRepairIsBoundedAndDoesNotQueueUnrepairedOrphans(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	if err := s.ReconcileFirewallMember(ctx, "node", true, true, 0); err != nil {
		t.Fatal(err)
	}
	clearDeliveredFirewallJobs(t, s)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO xnode_firewall_members(node_id)
 SELECT 'absent-'||LPAD(i::text,4,'0') FROM generate_series(1,$1::integer) AS series(i)`, firewallMemberRetirementBatch+1); err != nil {
		t.Fatal(err)
	}
	var version, sourceVersion uint64
	if err := s.db.QueryRowContext(ctx, `SELECT version,source_version FROM xnode_firewall_clock WHERE singleton=1`).Scan(&version, &sourceVersion); err != nil {
		t.Fatal(err)
	}
	for _, want := range []int{1, 0, 0} {
		if err := s.SweepFirewall(ctx, now); err != nil {
			t.Fatal(err)
		}
		var remaining, retained int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER(WHERE active=1),COUNT(*) FROM xnode_firewall_members WHERE node_id LIKE 'absent-%'`).Scan(&remaining, &retained); err != nil || remaining != want || retained != firewallMemberRetirementBatch+1 {
			t.Fatal("repair lost its bound or deleted membership history", remaining, retained, want, err)
		}
		// The unrepaired tail must not create deliveries merely because its old
		// active flag and force_sync still look pending during this bounded pass.
		if err := s.firewallWrite(ctx, func(tx *sql.Tx) error { return queueFirewallFleetTx(ctx, tx, now) }); err != nil {
			t.Fatal(err)
		}
		var jobs, healthy int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_firewall'`).Scan(&jobs); err != nil || jobs != 0 {
			t.Fatal("orphan membership queued delivery", jobs, err)
		}
		if err := s.db.QueryRowContext(ctx, `SELECT active FROM xnode_firewall_members WHERE node_id='node'`).Scan(&healthy); err != nil || healthy != 1 {
			t.Fatal("repair retired a retained node", healthy, err)
		}
		var after, sourceAfter uint64
		if err := s.db.QueryRowContext(ctx, `SELECT version,source_version FROM xnode_firewall_clock WHERE singleton=1`).Scan(&after, &sourceAfter); err != nil || after != version || sourceAfter != sourceVersion {
			t.Fatal("membership repair advanced clocks without a generation release", after, sourceAfter, err)
		}
	}
}
