package rollover

import (
	"math/big"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func CalculateUsage(purchase model.Purchase, threshold int, snapshot UsageSnapshot) model.RolloverUsageSummary {
	if snapshot.LimitBytes <= 0 || purchase.ValidUntil.Before(purchase.ValidFrom) {
		return model.RolloverUsageSummary{AlgorithmVersion: UsageAlgorithmVersion}
	}
	start, end := purchase.ValidFrom.UTC(), purchase.ValidUntil.UTC()
	anchor := start
	if snapshot.LastResetAt != nil && snapshot.LastResetAt.Before(end) {
		anchor = snapshot.LastResetAt.UTC()
	}
	return calculateUsageRange(threshold, snapshot, anchor, start, end)
}

func calculateUsageRange(threshold int, snapshot UsageSnapshot, anchor, start, end time.Time) model.RolloverUsageSummary {
	if snapshot.LimitBytes <= 0 || !end.After(start) {
		return model.RolloverUsageSummary{AlgorithmVersion: UsageAlgorithmVersion}
	}
	periods := cadencePeriods(anchor, start, end, snapshot.Strategy)
	usageByDay := make(map[string]int64, len(snapshot.Daily))
	for _, item := range snapshot.Daily {
		if item.Bytes > 0 {
			usageByDay[item.Date.UTC().Format(time.DateOnly)] += item.Bytes
		}
	}
	var allocated, used int64
	for _, period := range periods {
		full := end.Sub(start)
		if snapshot.Strategy != "NO_RESET" {
			full = cadenceAdvance(period.start, snapshot.Strategy).Sub(period.start)
		}
		if full <= 0 {
			full = end.Sub(start)
		}
		overlap := overlapDuration(period.start, period.end, start, end)
		if overlap <= 0 {
			continue
		}
		allowance := proportionalBytes(snapshot.LimitBytes, overlap.Nanoseconds(), full.Nanoseconds())
		periodUsed := int64(0)
		windowStart := period.start
		if windowStart.Before(start) {
			windowStart = start
		}
		windowEnd := period.end
		if windowEnd.After(end) {
			windowEnd = end
		}
		for day := dayStart(windowStart); day.Before(windowEnd); day = day.AddDate(0, 0, 1) {
			dayEnd := day.AddDate(0, 0, 1)
			portion := overlapDuration(day, dayEnd, windowStart, windowEnd)
			if portion <= 0 {
				continue
			}
			periodUsed += proportionalBytes(usageByDay[day.Format(time.DateOnly)], portion.Nanoseconds(), (24 * time.Hour).Nanoseconds())
		}
		if allowance < 0 {
			allowance = 0
		}
		if periodUsed > allowance {
			periodUsed = allowance
		}
		if allowance <= 0 {
			continue
		}
		allocated = sumBytes(allocated, allowance)
		used = sumBytes(used, periodUsed)
	}
	eligible := EligibleUnused(allocated, used, threshold)
	return model.RolloverUsageSummary{AllocatedBytes: allocated, UsedBytes: used, EligibleUnusedBytes: eligible, AlgorithmVersion: UsageAlgorithmVersion}
}

func strictlyAboveBPS(remaining, allowance int64, threshold int) bool {
	if remaining <= 0 || allowance <= 0 {
		return false
	}
	left := new(big.Int).Mul(big.NewInt(remaining), big.NewInt(10000))
	right := new(big.Int).Mul(big.NewInt(allowance), big.NewInt(int64(threshold)))
	return left.Cmp(right) > 0
}

func sumBytes(left, right int64) int64 {
	if right <= 0 {
		return left
	}
	if left >= 1<<63-1-right {
		return 1<<63 - 1
	}
	return left + right
}

func dayStart(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func overlapDuration(left, right, start, end time.Time) time.Duration {
	if left.Before(start) {
		left = start
	}
	if right.After(end) {
		right = end
	}
	if !right.After(left) {
		return 0
	}
	return right.Sub(left)
}

func proportionalBytes(value, numerator, denominator int64) int64 {
	if value <= 0 || numerator <= 0 || denominator <= 0 {
		return 0
	}
	result := new(big.Int).Mul(big.NewInt(value), big.NewInt(numerator))
	result.Quo(result, big.NewInt(denominator))
	if !result.IsInt64() {
		return value
	}
	return result.Int64()
}
