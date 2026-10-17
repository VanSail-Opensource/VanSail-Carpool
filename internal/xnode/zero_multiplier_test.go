package xnode_test

import (
	"context"
	"testing"
)

func TestNodeSettingsPersistExplicitZeroAndSixDecimalMultipliers(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	for _, multiplier := range []float64{0, 1, 0.000001, 1.123456, 1000} {
		node, settings, err := f.service.Node(ctx, f.node.ID)
		if err != nil {
			t.Fatal(err)
		}
		node.Multiplier = multiplier
		saved, err := f.service.SaveNode(ctx, node, settings)
		if err != nil || saved.Multiplier != multiplier {
			t.Fatalf("multiplier %v did not round-trip: %+v %v", multiplier, saved, err)
		}
	}
}
