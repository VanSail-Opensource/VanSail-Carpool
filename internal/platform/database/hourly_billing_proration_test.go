package database

import (
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestHourlyProrationUsesExactMinorUnits(t *testing.T) {
	base := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name     string
		price    int64
		duration time.Duration
		want     int64
	}{
		{"whole hour", 101, time.Hour, 101},
		{"half hour rounds up", 101, time.Minute * 30, 51},
		{"last nanosecond", 101, time.Nanosecond, 1},
		{"free explicit offer", 0, time.Nanosecond, 0},
		{"maximum integer", math.MaxInt64, time.Hour, math.MaxInt64},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := hourlyCharge(test.price, base, base.Add(test.duration))
			if err != nil || got != test.want {
				t.Fatalf("charge = %d, %v; want %d", got, err, test.want)
			}
		})
	}
	for _, duration := range []time.Duration{0, -1, time.Hour + 1} {
		if _, err := hourlyCharge(1, base, base.Add(duration)); err == nil {
			t.Fatalf("accepted invalid duration %s", duration)
		}
	}
	policy := model.DefaultHourlyPolicy()
	if _, err := model.HourlyActivationMinimum(policy, 0, math.MaxInt64); err == nil {
		t.Fatal("overflowed activation threshold")
	}
	if got, err := model.HourlyActivationMinimum(policy, 0, 101); err != nil || got != 16968 {
		t.Fatalf("eligibility = %d, %v", got, err)
	}
	local := base.Add(45 * time.Minute).In(time.FixedZone("fixture", 8*60*60))
	if got := hourlyBoundary(local); !got.Equal(base.Add(time.Hour)) || got.Location() != time.UTC {
		t.Fatalf("hour boundary did not use UTC: %s", got)
	}
	midnight := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	if got := hourlyBoundary(midnight.Add(-time.Nanosecond)); !got.Equal(midnight) {
		t.Fatalf("UTC midnight boundary = %s", got)
	}
	if charge, err := hourlyCharge(101, midnight.Add(-time.Nanosecond), midnight); err != nil || charge != 1 {
		t.Fatalf("last instant of UTC day = %d, %v", charge, err)
	}
}
