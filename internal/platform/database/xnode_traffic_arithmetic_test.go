package database

import (
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNativeUsageRetainsHistoricalCarryAcrossRetiredRates(t *testing.T) {
	var value nativeUsageValue
	steps := []struct {
		upload, download, multiplier, delta int64
		want                                nativeUsageValue
	}{
		{1, 2, 1_250_000, 3, nativeUsageValue{1, 2, 3, 750_000}},
		{80, 0, 0, 0, nativeUsageValue{81, 2, 3, 750_000}},
		{2, 1, xnode.MultiplierScale, 3, nativeUsageValue{83, 3, 6, 750_000}},
		{0, 4, xnode.MultiplierScale, 4, nativeUsageValue{83, 7, 10, 750_000}},
	}
	for index, step := range steps {
		next, delta, err := nextNativeUsage(value, step.upload, step.download, step.multiplier)
		if err != nil || delta != step.delta || next != step.want {
			t.Fatalf("step %d: next=%+v delta=%d error=%v", index, next, delta, err)
		}
		value = next
	}
}
