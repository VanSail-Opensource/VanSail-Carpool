package xnodemigration

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"testing"
)

func TestSourceHistoryRequiresAllSeriesAndUsesExactMultiplierUnits(t *testing.T) {
	stats := remnawave.UserStats{Categories: []string{"2026-10-14"}, SparklineData: []int64{9}, Series: []remnawave.NodeUsageSeries{{UUID: "a", Data: []int64{4}}, {UUID: "b", Data: []int64{5}}}}
	values, err := History(42, stats, map[string]int64{"a": 1250000, "b": 2000000})
	if err != nil || len(values) != 2 || values[0].WeightedBytes != 5 || values[1].WeightedBytes != 10 {
		t.Fatalf("history=%+v,%v", values, err)
	}
	stats.Series = stats.Series[:1]
	if _, err = History(42, stats, map[string]int64{"a": 1250000}); err == nil {
		t.Fatal("truncated top-node series accepted as complete accounting")
	}
}
