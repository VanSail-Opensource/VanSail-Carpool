package app

import (
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

// Remnawave currently rejects values above 10, despite its API schema not
// declaring an upper bound. Reconciliation below rejects any truncated series
// so rollover and migration never settle from incomplete per-node usage.
const rolloverStatsTopNodesLimit = 10

func validateRolloverStatistics(user *remnawave.User, stats *remnawave.UserStats, start, end time.Time) ([]time.Time, error) {
	if user == nil || !validRolloverTrafficStrategy(user.TrafficLimitStrategy) || stats == nil ||
		stats.Categories == nil || stats.SparklineData == nil || stats.Series == nil ||
		len(stats.Categories) != len(stats.SparklineData) {
		return nil, rollover.ErrPerNodeUsageUnavailable
	}
	startDay, endDay := rolloverStatisticDay(start), rolloverStatisticDay(end)
	if endDay.Before(startDay) {
		return nil, rollover.ErrPerNodeUsageUnavailable
	}
	categories := make([]time.Time, 0, len(stats.Categories))
	seenCategories := make(map[string]struct{}, len(stats.Categories))
	for index, category := range stats.Categories {
		date, err := time.Parse(time.DateOnly, category)
		if err != nil || date.Before(startDay) || date.After(endDay) {
			return nil, rollover.ErrPerNodeUsageUnavailable
		}
		if _, exists := seenCategories[category]; exists || stats.SparklineData[index] < 0 {
			return nil, rollover.ErrPerNodeUsageUnavailable
		}
		seenCategories[category] = struct{}{}
		categories = append(categories, date)
	}
	if !strictlyIncreasing(categories) {
		return nil, rollover.ErrPerNodeUsageUnavailable
	}
	rawBuckets := make([]int64, len(categories))
	seenSeries := make(map[string]struct{}, len(stats.Series))
	for _, series := range stats.Series {
		if series.UUID == "" || len(series.Data) != len(categories) {
			return nil, rollover.ErrPerNodeUsageUnavailable
		}
		if _, exists := seenSeries[series.UUID]; exists {
			return nil, rollover.ErrPerNodeUsageUnavailable
		}
		seenSeries[series.UUID] = struct{}{}
		for index, value := range series.Data {
			if value < 0 || rawBuckets[index] > 1<<63-1-value {
				return nil, rollover.ErrPerNodeUsageUnavailable
			}
			rawBuckets[index] += value
		}
	}
	for index, actual := range rawBuckets {
		if actual != stats.SparklineData[index] {
			return nil, fmt.Errorf("%w: node-series bucket %d does not match aggregate", rollover.ErrPerNodeUsageUnavailable, index)
		}
	}
	return categories, nil
}

func validRolloverTrafficStrategy(strategy remnawave.TrafficLimitStrategy) bool {
	switch strategy {
	case remnawave.TrafficNoReset, remnawave.TrafficDaily, remnawave.TrafficWeekly, remnawave.TrafficMonthly, remnawave.TrafficMonthlyRolling:
		return true
	default:
		return false
	}
}

func rolloverStatisticDay(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func strictlyIncreasing(values []time.Time) bool {
	for index := 1; index < len(values); index++ {
		if !values[index].After(values[index-1]) {
			return false
		}
	}
	return true
}
