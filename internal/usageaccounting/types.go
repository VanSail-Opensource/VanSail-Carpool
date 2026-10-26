// Package usageaccounting shares quota cadence and exact refund allocation.
// It never issues unused-traffic credits or changes a wallet.
package usageaccounting

import "time"

const UsageAlgorithmVersion = "cadence-v4"
const NativeUsageAlgorithmVersion = "native-term-v1"

type DailyUsage struct {
	Date  time.Time
	Bytes int64
}
type Summary struct {
	AllocatedBytes, UsedBytes int64
	AlgorithmVersion          string
}
type UsageSnapshot struct {
	LimitBytes          int64
	Strategy            string
	LastResetAt         *time.Time
	CurrentUsedBytes    *int64
	Daily               []DailyUsage
	WeightedUsedBytes   int64
	NodeSeriesAvailable bool
	TermUsageExact      bool
}
