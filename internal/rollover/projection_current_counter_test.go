package rollover

import (
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCalculateUsageUsesWeightedDailyUsageInsteadOfCurrentCounter(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reset := start.AddDate(0, 0, 1)
	currentUsed := int64(0)
	summary := CalculateUsage(model.Purchase{ValidFrom: start, ValidUntil: start.AddDate(0, 0, 2)}, 0, UsageSnapshot{
		LimitBytes: 1_000, Strategy: "DAY", LastResetAt: &reset, CurrentUsedBytes: &currentUsed,
		Daily: []DailyUsage{{Date: start, Bytes: 100}, {Date: reset, Bytes: 900}}, NodeSeriesAvailable: true,
	})
	if summary.UsedBytes != 1_000 || summary.EligibleUnusedBytes != 1_000 {
		t.Fatalf("summary = %+v, want weighted daily used=1000 eligible=1000", summary)
	}
}

func TestCadenceBoundaries(t *testing.T) {
	monthEnd := time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)
	if got, want := cadenceAdvance(monthEnd, "MONTH"), time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("monthly advance = %s, want %s", got, want)
	}
	if got, want := cadenceAdvance(monthEnd, "MONTH_ROLLING"), time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("rolling monthly advance = %s, want %s", got, want)
	}
	februaryEnd := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	if got, want := cadencePrevious(februaryEnd, "MONTH"), monthEnd; !got.Equal(want) {
		t.Fatalf("monthly previous = %s, want %s", got, want)
	}
}
