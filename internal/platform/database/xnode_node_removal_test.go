package database

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
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
	if _, err := s.FinishXNodeRemoval(ctx, "node", version(), identity.UserID, "pending"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("pending accounting: %v", err)
	}
	if err := s.SealXNodePeriodNode(ctx, period.ID, "node", 0); err != nil {
		t.Fatal(err)
	}
	if retired, err := s.FinishXNodeRemoval(ctx, "node", version(), identity.UserID, "finished"); err != nil || !retired {
		t.Fatal(retired, err)
	}
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
