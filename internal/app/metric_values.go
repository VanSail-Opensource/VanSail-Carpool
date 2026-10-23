package app

import "math"

func roundedNonNegativeInt64(value float64) int64 {
	if value <= 0 || math.IsNaN(value) {
		return 0
	}
	const maximum = int64(1<<63 - 1)
	if value >= float64(maximum) {
		return maximum
	}
	return int64(math.Round(value))
}
