package rollover

import (
	"math/big"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ProjectUsage returns the current weighted usage and a full-term forecast.
// The eligibility boundary is strict: remaining traffic must be greater than
// the configured basis-point threshold.
func ProjectUsage(purchase model.Purchase, threshold int, snapshot UsageSnapshot, asOf time.Time) model.RolloverProjection {
	start := purchase.ValidFrom.UTC()
	now := asOf.UTC()
	if now.Before(start) {
		now = start
	}
	if now.After(purchase.ValidUntil.UTC()) {
		now = purchase.ValidUntil.UTC()
	}
	anchor := start
	if snapshot.LastResetAt != nil && snapshot.LastResetAt.Before(now) {
		anchor = snapshot.LastResetAt.UTC()
	}
	current := calculateUsageRange(threshold, snapshot, anchor, start, now)
	full := CalculateUsage(purchase, threshold, snapshot)
	actual := snapshot.WeightedUsedBytes
	projected := projectFullTerm(actual, now.Sub(start), purchase.ValidUntil.Sub(start))
	maximum := maximumAllowableUsage(full.AllocatedBytes, threshold)
	remaining := full.AllocatedBytes - projected
	if remaining < 0 {
		remaining = 0
	}
	daysLeft := int64((purchase.ValidUntil.Sub(now) + 24*time.Hour - 1).Hours() / 24)
	if daysLeft < 1 {
		daysLeft = 1
	}
	var maximumDailyUsage *int64
	if actual <= maximum {
		value := (maximum - actual) / daysLeft
		maximumDailyUsage = pointer(value)
	}
	result := model.RolloverProjection{
		PurchaseID: purchase.ID, Paid: model.TXBMoney(purchase.PriceTXBMinor),
		AutoRenewalEnabled: purchase.AutoRenewEnabled, MinimumRemainingBPS: threshold,
		ActualUsedTrafficBytes: pointer(actual), ProjectedFullTermUsageBytes: pointer(projected),
		MaximumAllowableUsageBytes: pointer(maximum),
		MaximumDailyUsageBytes:     maximumDailyUsage,
		Term:                       pointerWindow(buildWindow(start, now, current, purchase.PriceTXBMinor)),
		LastResetPeriod:            pointerWindow(buildLastWindow(start, now, snapshot, purchase.PriceTXBMinor, threshold)),
	}
	if projected <= maximum && strictlyAboveBPS(full.AllocatedBytes-projected, full.AllocatedBytes, threshold) {
		result.PredictedRollover = pointerMoney(model.TXBMoney(CreditMinor(purchase.PriceTXBMinor, remaining, full.AllocatedBytes)))
		return result
	}
	reduction := projected - maximum
	if reduction < 0 {
		reduction = 0
	}
	result.RequiredReductionBytes = pointer(reduction)
	result.RequiredDailyReductionBytes = pointer(ceilDivide(reduction, daysLeft))
	return result
}

func buildLastWindow(start, end time.Time, snapshot UsageSnapshot, paid int64, threshold int) model.RolloverWindow {
	lastStart, lastEnd := latestResetBounds(start, end, snapshot)
	last := calculateUsageRange(threshold, snapshot, lastStart, lastStart, lastEnd)
	return buildWindow(lastStart, lastEnd, last, paid)
}

func buildWindow(start, end time.Time, summary model.RolloverUsageSummary, paid int64) model.RolloverWindow {
	remaining := summary.AllocatedBytes - summary.UsedBytes
	if remaining < 0 {
		remaining = 0
	}
	eligible := summary.EligibleUnusedBytes
	return model.RolloverWindow{Start: start.UTC(), End: end.UTC(), AllocatedTrafficBytes: summary.AllocatedBytes,
		UsedTrafficBytes: summary.UsedBytes, RemainingTrafficBytes: remaining, EligibleUnusedBytes: eligible,
		Rollover: model.TXBMoney(CreditMinor(paid, eligible, summary.AllocatedBytes))}
}

func latestResetBounds(termStart, termEnd time.Time, snapshot UsageSnapshot) (time.Time, time.Time) {
	if !termEnd.After(termStart) || snapshot.Strategy == "NO_RESET" {
		return termStart, termEnd
	}
	periodStart := termStart
	if snapshot.LastResetAt != nil && snapshot.LastResetAt.Before(termEnd) {
		periodStart = snapshot.LastResetAt.UTC()
	} else if periods := cadencePeriods(termStart, termStart, termEnd, snapshot.Strategy); len(periods) > 0 {
		periodStart = periods[len(periods)-1].start
	}
	periodEnd := cadenceAdvance(periodStart, snapshot.Strategy)
	if periodStart.Before(termStart) {
		periodStart = termStart
	}
	if periodEnd.After(termEnd) {
		periodEnd = termEnd
	}
	return periodStart, periodEnd
}

func projectFullTerm(actual int64, elapsed, full time.Duration) int64 {
	if actual <= 0 || elapsed <= 0 || full <= 0 {
		return 0
	}
	return proportionalBytes(actual, full.Nanoseconds(), elapsed.Nanoseconds())
}

func maximumAllowableUsage(allocated int64, threshold int) int64 {
	if allocated <= 0 {
		return 0
	}
	thresholdBytes := new(big.Int).Mul(big.NewInt(allocated), big.NewInt(int64(threshold)))
	thresholdBytes.Quo(thresholdBytes, big.NewInt(10000))
	result := allocated - thresholdBytes.Int64() - 1
	if result < 0 {
		return 0
	}
	return result
}

func ceilDivide(numerator, denominator int64) int64 {
	if numerator <= 0 || denominator <= 0 {
		return 0
	}
	return (numerator + denominator - 1) / denominator
}

func pointer(value int64) *int64                                     { return &value }
func pointerMoney(value model.Money) *model.Money                    { return &value }
func pointerWindow(value model.RolloverWindow) *model.RolloverWindow { return &value }
