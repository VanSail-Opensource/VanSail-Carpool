package database

import (
	"fmt"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func BenchmarkTrafficBatchNative(b *testing.B) {
	for _, batched := range []bool{false, true} {
		name := "individual"
		if batched {
			name = "batched"
		}
		b.Run(name, func(b *testing.B) { benchmarkTrafficBatchNative(b, batched) })
	}
}

func benchmarkTrafficBatchNative(b *testing.B, batched bool) {
	const nodes, users = 32, 64
	s, items, period := trafficBatchBenchmarkFixture(b, nodes, users)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for i := range items {
			item := &items[i]
			item.Sequence++
			item.Report.ID = fmt.Sprintf("%s:%d", item.AgentID, item.Sequence)
			item.Digest = item.Report.ID
		}
		if batched {
			outcomes, err := s.RecordXNodeTrafficBatch(b.Context(), items)
			if err != nil {
				b.Fatal(err)
			}
			for _, outcome := range outcomes {
				if outcome != nil {
					b.Fatal(outcome)
				}
			}
		} else {
			for _, item := range items {
				if err := s.RecordXNodeTraffic(b.Context(), item.NodeID, item.AgentID, item.Sequence, item.Digest, item.Report); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	b.StopTimer()
	value, err := s.XNodePeriod(b.Context(), period)
	want := int64(nodes) * (int64(b.N) * 3 * 1250000 / xnode.MultiplierScale)
	if err != nil || value.UsedBytes != want {
		b.Fatal("benchmark accounting mismatch", value.UsedBytes, want, err)
	}
	b.ReportMetric(nodes, "reports/op")
	b.ReportMetric(nodes*users, "samples/op")
}
