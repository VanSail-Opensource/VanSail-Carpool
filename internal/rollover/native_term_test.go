package rollover

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"math"
	"testing"
	"time"
)

func TestNativeTermDoesNotProrateOwnedUsageTwiceAtIntradayBoundaries(t *testing.T) {
	start := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	purchase := model.Purchase{ValidFrom: start, ValidUntil: start.Add(24 * time.Hour)}
	snapshot := UsageSnapshot{LimitBytes: 1000, Strategy: "NO_RESET", TermUsageExact: true, WeightedUsedBytes: 400, Daily: []DailyUsage{{Date: start, Bytes: 400}}}
	got := CalculateUsage(purchase, 0, snapshot)
	if got.UsedBytes != 400 || got.EligibleUnusedBytes != 600 || got.AlgorithmVersion != "native-term-v1" {
		t.Fatalf("exact term=%+v", got)
	}
	if got := ceilDivide(math.MaxInt64, 2); got != math.MaxInt64/2+1 {
		t.Fatalf("daily reduction overflow: %d", got)
	}
}
