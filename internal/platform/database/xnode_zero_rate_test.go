package database

import (
	"context"
	"fmt"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"testing"
	"time"
)

func TestZeroNodeRateRetainsRawUsageAndSettledRateHistory(t *testing.T) {
	store, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	record := func(sequence uint64, bytes uint64) {
		t.Helper()
		report := wire.TrafficReport{ID: fmt.Sprintf("agent:%d", sequence), Traffic: wire.Traffic{int(identity.ProxyUserID): {bytes, 0}}, Samples: []wire.TrafficSample{{UserID: int(identity.ProxyUserID), AccountingPeriod: period.ID, From: at, Through: at, UploadBytes: bytes}}}
		if err := store.RecordXNodeTraffic(ctx, "node", "agent", sequence, fmt.Sprint(sequence), report); err != nil {
			t.Fatal(err)
		}
	}
	record(1, 4)
	if err := store.SetXNodeRate(ctx, "node", "agent", 1, 0); err != nil {
		t.Fatal(err)
	}
	record(2, 80)
	value, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || value.UsedBytes != 5 {
		t.Fatalf("free traffic changed charged usage: %+v %v", value, err)
	}
	usage, err := store.XNodeUsage(ctx, identity.ProxyUserID, at, at)
	if err != nil || len(usage) != 1 || usage[0].UploadBytes != 84 || usage[0].WeightedBytes != 5 {
		t.Fatalf("raw free-node accounting was lost: %+v %v", usage, err)
	}
	// A stale flush result cannot rewrite the already committed zero-price range.
	if err = store.SetXNodeRate(ctx, "node", "agent", 1, 2000000); err != nil {
		t.Fatal(err)
	}
	var original, free int64
	if err = store.db.QueryRowContext(ctx, "SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='agent' AND from_sequence=1").Scan(&original); err != nil {
		t.Fatal(err)
	}
	if err = store.db.QueryRowContext(ctx, "SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id='agent' AND from_sequence=2").Scan(&free); err != nil {
		t.Fatal(err)
	}
	if original != 1250000 || free != 0 {
		t.Fatal("settled rate boundaries were rewritten")
	}
	if err = store.SetXNodeRate(ctx, "node", "agent", 2, -1); err == nil {
		t.Fatal("negative multiplier accepted")
	}
}
