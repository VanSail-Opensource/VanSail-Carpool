package xnode_test

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNodeSaveNormalizesHistoricalScaleWithoutAgentFlush(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_nodes SET agent_id='offline-agent' WHERE id=$1`, f.node.ID); err != nil {
		t.Fatal(err)
	}
	for _, obsolete := range []int64{0, -1, 12_500_000} {
		node, settings, err := f.service.Node(ctx, f.node.ID)
		if err != nil {
			t.Fatal(err)
		}
		node.MultiplierFP = obsolete
		saved, err := f.service.SaveNode(ctx, node, settings)
		if err != nil || saved.MultiplierFP != xnode.MultiplierScale {
			t.Fatal("obsolete input altered billing or required online flush", saved, err)
		}
	}
}
