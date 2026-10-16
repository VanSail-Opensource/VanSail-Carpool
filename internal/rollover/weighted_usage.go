package rollover

import (
	"math/big"
	"time"
)

const multiplierScale int64 = 1_000_000

// NodeUsageSeries is an in-memory adapter of Remnawave's raw per-node series.
// It is deliberately not part of any persisted model.
type NodeUsageSeries struct {
	UUID         string
	MultiplierFP int64
	Data         []int64
}

// WeightedNodeUsage converts node traffic to the same weighted byte total used
// by the dashboard traffic bar, using fixed-point multiplier arithmetic.
func WeightedNodeUsage(categories []time.Time, series []NodeUsageSeries) (daily []DailyUsage, total int64) {
	values := make([]int64, len(categories))
	for _, node := range series {
		for index, raw := range node.Data {
			if index >= len(values) || raw <= 0 || node.MultiplierFP <= 0 {
				continue
			}
			weighted := new(big.Int).Mul(big.NewInt(raw), big.NewInt(node.MultiplierFP))
			weighted.Quo(weighted, big.NewInt(multiplierScale))
			if weighted.IsInt64() {
				values[index] = saturatingAdd(values[index], weighted.Int64())
			} else {
				values[index] = 1<<63 - 1
			}
		}
	}
	daily = make([]DailyUsage, 0, len(categories))
	for index, value := range values {
		if value <= 0 || index >= len(categories) || categories[index].IsZero() {
			continue
		}
		daily = append(daily, DailyUsage{Date: categories[index].UTC(), Bytes: value})
		total = saturatingAdd(total, value)
	}
	return daily, total
}

func saturatingAdd(left, right int64) int64 {
	if right <= 0 {
		return left
	}
	if left >= 1<<63-1-right {
		return 1<<63 - 1
	}
	return left + right
}
