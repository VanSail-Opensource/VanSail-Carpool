package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"testing"
	"time"
)

func TestLegacySnapshotsKeepResetTailsAndNeverReimportTheBaseline(t *testing.T) {
	store, identity, old := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	reset := at.Add(-time.Hour)
	sync := func(period string, used, lifetime int64, resetAt time.Time, offset time.Duration) {
		t.Helper()
		err := store.SyncXNodeLegacyUsage(ctx, xnode.LegacyUsage{ProxyUserID: identity.ProxyUserID, PeriodID: period, UsedBytes: used, LifetimeBytes: lifetime, LastResetAt: &resetAt, SyncedAt: at.Add(offset)})
		if err != nil {
			t.Fatal(err)
		}
	}
	sync(old.ID, 100, 1000, reset, 0)
	old.Status = "closing"
	if err := store.SaveXNodePeriod(ctx, old); err != nil {
		t.Fatal(err)
	}
	next := old
	next.ID = "next-period"
	next.StartedAt = at
	next.Status = "open"
	if err := store.SaveXNodePeriod(ctx, next); err != nil {
		t.Fatal(err)
	}
	sync(next.ID, 110, 1010, reset, time.Second)
	sync(next.ID, 110, 1010, reset, time.Second) // replay
	a, _ := store.XNodePeriod(ctx, old.ID)
	b, _ := store.XNodePeriod(ctx, next.ID)
	if a.UsedBytes != 110 || b.UsedBytes != 0 {
		t.Fatalf("baseline charged twice: old=%d,new=%d", a.UsedBytes, b.UsedBytes)
	}
	nextReset := at.Add(2 * time.Second)
	sync(next.ID, 5, 1020, nextReset, 3*time.Second) // 5 old bytes and 5 new bytes
	a, _ = store.XNodePeriod(ctx, old.ID)
	b, _ = store.XNodePeriod(ctx, next.ID)
	if a.UsedBytes != 115 || b.UsedBytes != 5 {
		t.Fatalf("reset tail lost: old=%d,new=%d", a.UsedBytes, b.UsedBytes)
	}
	sync(next.ID, 100, 1000, reset, -time.Second) // stale preview after live sync
	a, _ = store.XNodePeriod(ctx, old.ID)
	b, _ = store.XNodePeriod(ctx, next.ID)
	if a.UsedBytes != 115 || b.UsedBytes != 5 {
		t.Fatal("stale export changed accounting")
	}
}

func TestLegacyHistoryDoesNotChargeQuotaAndReconcilesMonotonically(t *testing.T) {
	store, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	values := []xnode.LegacyHistory{{ProxyUserID: identity.ProxyUserID, NodeID: "historical-deleted-node", Date: "2026-10-14", RawBytes: 100, WeightedBytes: 125}}
	for i := 0; i < 2; i++ {
		if err := store.SaveXNodeLegacyHistory(ctx, values); err != nil {
			t.Fatal(err)
		}
	}
	current, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || current.UsedBytes != 0 {
		t.Fatal("historical series charged the aggregate twice")
	}
	values[0].RawBytes = 90
	if err := store.SaveXNodeLegacyHistory(ctx, values); err == nil {
		t.Fatal("decreasing source counter accepted")
	}
	rows, err := store.XNodeLegacyHistory(ctx, identity.ProxyUserID, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))
	if err != nil || len(rows) != 1 || rows[0].RawBytes != 100 {
		t.Fatalf("non-atomic history=%+v,%v", rows, err)
	}
}

func TestImportedNodePublishesLegacyOwnershipAtomicallyAndPreservesResumeEdits(t *testing.T) {
	store, _, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	if err := store.SaveXNodeMigration(ctx, xnode.MigrationState{Phase: "active"}); err != nil {
		t.Fatal(err)
	}
	node := xnode.Node{ID: "imported", Name: "Imported", ConfigID: "config", MultiplierFP: 1000000, SettingsCipher: "encrypted", Enabled: true}
	checkpoint := xnode.MigrationNode{NodeID: node.ID, LegacyNodeID: node.ID, Phase: "legacy", SourceDigest: "source"}
	if err := store.ImportXNodeMigrationNode(ctx, node, checkpoint); err != nil {
		t.Fatal(err)
	}
	state, err := store.XNodeMigration(ctx)
	if err != nil || len(state.Nodes) != 1 || state.Nodes[0].Phase != "legacy" {
		t.Fatalf("ownership=%+v,%v", state, err)
	}
	checkpoint.Phase = "native"
	if err := store.SaveXNodeMigrationNode(ctx, checkpoint); err != nil {
		t.Fatal(err)
	}
	node.Name = "Operator edit"
	if err := store.SaveXNodeNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	node.Name = "Original"
	checkpoint.Phase = "legacy"
	if err := store.ImportXNodeMigrationNode(ctx, node, checkpoint); err != nil {
		t.Fatal(err)
	}
	current, _ := store.XNodeNode(ctx, node.ID)
	state, _ = store.XNodeMigration(ctx)
	if current.Name != "Operator edit" || state.Nodes[0].Phase != "native" {
		t.Fatal("resumption overwrote operator edits or ownership")
	}
}
