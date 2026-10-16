package rollover

import (
	"math"
	"testing"
	"time"
)

func TestWeightedNodeUsageUsesFixedPointMultipliersWithoutPersistingSeries(t *testing.T) {
	start := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	daily, total := WeightedNodeUsage([]time.Time{start, start.AddDate(0, 0, 1)}, []NodeUsageSeries{
		{UUID: "node-a", MultiplierFP: 1_500_000, Data: []int64{100, 200}},
		{UUID: "node-b", MultiplierFP: 500_000, Data: []int64{100, 200}},
	})
	if total != 600 || len(daily) != 2 || daily[0].Bytes != 200 || daily[1].Bytes != 400 {
		t.Fatalf("weighted usage = (%+v, %d), want 200/400/600", daily, total)
	}
}

func TestWeightedUsageNeverDropsOverflowOrTurnsRoundingIntoUsage(t *testing.T) {
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name                  string
		raw, multiplier, want int64
	}{
		{"fractional byte", 1, 1, 0},
		{"overflow", math.MaxInt64, 2_000_000, math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, total := WeightedNodeUsage([]time.Time{day}, []NodeUsageSeries{{Data: []int64{tc.raw}, MultiplierFP: tc.multiplier}})
			if total != tc.want {
				t.Fatalf("weighted bytes = %d, want %d", total, tc.want)
			}
		})
	}
}
