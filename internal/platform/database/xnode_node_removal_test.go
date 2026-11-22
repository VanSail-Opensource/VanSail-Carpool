package database

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"testing"
	"time"
)

func TestNodeRemovalPrunesBindingsAndRetainsHistory(t *testing.T) {
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	version := func() int64 {
		v, _, err := s.XNodeCompileVersion(ctx, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if err := s.ReconcileFirewallMember(ctx, "node", true, false, 0); err != nil {
		t.Fatal(err)
	}
	var ordinal int64
	if err := s.db.QueryRowContext(ctx, `SELECT ordinal FROM xnode_firewall_members WHERE node_id='node'`).Scan(&ordinal); err != nil {
		t.Fatal(err)
	}
	assertMembership := func(want int) {
		t.Helper()
		var currentOrdinal int64
		var active int
		if err := s.db.QueryRowContext(ctx, `SELECT ordinal,active FROM xnode_firewall_members WHERE node_id='node'`).Scan(&currentOrdinal, &active); err != nil || active != want || currentOrdinal != ordinal {
			t.Fatal("removal changed the membership fence or historical ordinal", currentOrdinal, active, err)
		}
	}
	group := xnode.Group{ID: "disabled", Name: "Disabled", Enabled: false, Bindings: []xnode.GroupBinding{{NodeID: "node", InboundTag: "paid"}}}
	if err := s.SaveXNodeGroup(ctx, group); err != nil {
		t.Fatal(err)
	}
	if err := s.TrackXNodePeriodNode(ctx, period.ID, "node"); err != nil {
		t.Fatal(err)
	}
	if err := s.PrepareXNodeRemoval(ctx, "node", version()); err != nil {
		t.Fatal(err)
	}
	assertMembership(1) // Preparation still retains the host's enforcement obligations.
	groups, err := s.XNodeGroups(ctx)
	if err != nil || len(groups) != 1 || len(groups[0].Bindings) != 0 || groups[0].Enabled {
		t.Fatalf("bindings survived removal: %+v %v", groups, err)
	}
	removing, err := s.XNodeNode(ctx, "node")
	if err != nil || removing.RemovingAt == nil || removing.Enabled {
		t.Fatalf("missing durable admission fence: %+v %v", removing, err)
	}
	if err := s.TrackXNodePeriodNode(ctx, period.ID, "node"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("late admission: %v", err)
	}
	account := xnode.PAYGAccount{ProxyUserID: identity.ProxyUserID, UserID: identity.UserID, GroupID: group.ID, NodeID: "node", PeriodID: period.ID, UnitPriceMinor: 100}
	if err := s.SaveXNodePAYGAccount(ctx, account); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("late traffic-mode registration: %v", err)
	}
	if _, err := s.FinishXNodeRemoval(ctx, "node", version(), identity.UserID, "pending"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("pending accounting: %v", err)
	}
	assertMembership(1)
	if err := s.SealXNodeRemovalPeriodNode(ctx, period, "node", wire.TrafficBarrier{AgentID: "agent", AccountingPeriod: period.ID, Quiesced: true}); err != nil {
		t.Fatal(err)
	}
	// A duplicate operation audit fails after the lifecycle and membership
	// writes. Neither half of historical retirement may escape that rollback.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO audit_events(id,action,target_type,target_id,created_at) VALUES('node-removal:rollback','fixture','xnode_node','node',$1)`, stamp(time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FinishXNodeRemoval(ctx, "node", version(), identity.UserID, "rollback"); err == nil {
		t.Fatal("duplicate removal audit did not fail the transaction")
	}
	assertMembership(1)
	if current, err := s.XNodeNode(ctx, "node"); err != nil || current.RetiredAt != nil {
		t.Fatal("failed finalization partially retired the node", current, err)
	}
	if retired, err := s.FinishXNodeRemoval(ctx, "node", version(), identity.UserID, "finished"); err != nil || !retired {
		t.Fatal(retired, err)
	}
	assertMembership(0)
	historical, err := s.XNodeNode(ctx, "node")
	if err != nil || historical.RetiredAt == nil || historical.Enabled || len(historical.TokenHash) != 0 {
		t.Fatal(historical, err)
	}
	active, err := s.XNodeNodes(ctx)
	if err != nil || len(active) != 0 {
		t.Fatal(active, err)
	}
	participants, err := s.XNodePeriodNodes(ctx, period.ID)
	if err != nil || len(participants) != 1 || !participants[0].Flushed {
		t.Fatal(participants, err)
	}
	group.Bindings = []xnode.GroupBinding{{NodeID: "node", InboundTag: "paid"}}
	if err = s.SaveXNodeGroup(ctx, group); err == nil {
		t.Fatal("retired node rebound")
	}
}

func TestUnusedNodeRemovalAndConcurrentBindingFence(t *testing.T) {
	s, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	node := xnode.Node{ID: "unused", Name: "Unused", ConfigID: "config", SettingsCipher: "encrypted", Enabled: false, MultiplierFP: xnode.MultiplierScale}
	if err := s.SaveXNodeNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	version, _, _ := s.XNodeCompileVersion(ctx, time.Now())
	if err := s.PrepareXNodeRemoval(ctx, node.ID, version); err != nil {
		t.Fatal(err)
	}
	version, _, _ = s.XNodeCompileVersion(ctx, time.Now())
	group := xnode.Group{ID: "race", Name: "Concurrent", Bindings: []xnode.GroupBinding{{NodeID: node.ID, InboundTag: "paid"}}}
	if err := s.SaveXNodeGroup(ctx, group); err == nil {
		t.Fatal("removing node rebound")
	}
	node.Enabled = true
	if err := s.SaveXNodeNode(ctx, node); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("removing node reactivated: %v", err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FinishXNodeRemoval(ctx, node.ID, version, identity.UserID, "stale"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("stale removal: %v", err)
	}
	version, _, _ = s.XNodeCompileVersion(ctx, time.Now())
	if retired, err := s.FinishXNodeRemoval(ctx, node.ID, version, identity.UserID, "unused"); err != nil || retired {
		t.Fatal(retired, err)
	}
	if _, err := s.XNodeNode(ctx, node.ID); !errors.Is(err, xnode.ErrNotFound) {
		t.Fatal(err)
	}
}
