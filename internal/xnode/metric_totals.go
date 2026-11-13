package xnode

import "math"

// Counts are nonnegative and validated per report. A fleet sum may exceed the
// legacy int64 fields; saturating preserves conservative access comparisons.
func addConnectionCount(total, count int64) int64 {
	if count > math.MaxInt64-total {
		return math.MaxInt64
	}
	return total + count
}

// Individual rates are finite, but their sum can overflow during projection.
func addMetricRate(total, rate float64) float64 {
	next := total + rate
	if math.IsInf(next, 1) {
		return math.MaxFloat64
	}
	return next
}
