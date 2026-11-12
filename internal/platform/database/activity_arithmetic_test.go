package database

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestSubscriptionExtensionRejectsDurationWraparound(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	// This sum can arise from multiple individually valid pending credits.
	// Multiplying first wraps to a positive duration and would silently shorten it.
	tooManyHours := 3 * int(math.MaxInt64/int64(time.Hour))
	if _, err := addSubscriptionHours(start, tooManyHours); !errors.Is(err, activity.ErrInvalidInput) {
		t.Fatalf("overflowing extension: %v", err)
	}
	end, err := addSubscriptionHours(start, 3650*24)
	if err != nil || !end.Equal(start.Add(3650*24*time.Hour)) {
		t.Fatalf("ordinary extension = %s, %v", end, err)
	}
}

func TestForecastMinorConversionRejectsExclusiveUpperBoundary(t *testing.T) {
	t.Parallel()
	for _, value := range []float64{float64(math.MaxInt64), math.Inf(1), math.NaN()} {
		if got := minorPointer(value, false); got != nil {
			t.Fatalf("out-of-range forecast %g became %s", value, *got)
		}
	}
	if got := minorPointer(math.Nextafter(float64(math.MaxInt64), 0), false); got == nil || *got != "9223372036854774784" {
		t.Fatalf("largest representable in-range forecast = %v", got)
	}
	if got := minorPointer(float64(math.MinInt64), false); got == nil || *got != "-9223372036854775808" {
		t.Fatalf("inclusive lower boundary = %v", got)
	}
}
