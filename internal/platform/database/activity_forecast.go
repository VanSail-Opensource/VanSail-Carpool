package database

import (
	"context"
	"math"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

// LuckyDrawForecast is a current-catalog estimate, expressed in TXB minor units.
type LuckyDrawForecast struct {
	Entries                 int     `json:"entries"`
	IncomeMinor             string  `json:"incomeMinor"`
	ExpectedExpenseMinMinor *string `json:"expectedExpenseMinMinor"`
	ExpectedExpenseMaxMinor *string `json:"expectedExpenseMaxMinor"`
	PossibleExpenseMinMinor *string `json:"possibleExpenseMinMinor"`
	PossibleExpenseMaxMinor *string `json:"possibleExpenseMaxMinor"`
	BreakEvenMinMinor       *string `json:"breakEvenMinMinor"`
	BreakEvenMaxMinor       *string `json:"breakEvenMaxMinor"`
	AverageBalanceMinor     *string `json:"averageBalanceMinor"`
	MultiplierUnavailable   bool    `json:"multiplierUnavailable"`
	CouponOneTermOnly       bool    `json:"couponOneTermOnly"`
}

func minorPointer(value float64, ceil bool) *string {
	if ceil {
		value = math.Ceil(value)
	} else {
		value = math.Floor(value)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || value > math.MaxInt64 || value < math.MinInt64 {
		return nil
	}
	result := strconv.FormatInt(int64(value), 10)
	return &result
}
func breakEvenPointer(value float64, entries int) *string {
	if entries <= 0 {
		return nil
	}
	return minorPointer(math.Max(1, math.Ceil(value/float64(entries))), true)
}

// LuckyDrawForecast prices a saved configuration using current local catalog and balances.
func (s *Store) LuckyDrawForecast(ctx context.Context, id string) (LuckyDrawForecast, error) {
	draw, err := s.LuckyDrawByID(ctx, id)
	if err != nil {
		return LuckyDrawForecast{}, err
	}
	prices, err := s.drawPriceContext(ctx)
	if err != nil {
		return LuckyDrawForecast{}, err
	}
	entries := draw.ExpectedParticipation
	if draw.Kind == "raffle" {
		entries = draw.Threshold
	}
	result := LuckyDrawForecast{Entries: entries}
	income := int64(entries) * draw.FeeMinor
	if draw.Kind == "raffle" && draw.Status != "draft" {
		var sold int
		var collected int64
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(fee_minor),0) FROM activity_raffle_tickets
   WHERE draw_id=? AND status IN ('active','settled')`, id).Scan(&sold, &collected)
		if err != nil {
			return result, err
		}
		income = collected + int64(max(0, entries-sold))*draw.FeeMinor
	}
	result.IncomeMinor = strconv.FormatInt(income, 10)
	if prices.avgBalance != nil {
		result.AverageBalanceMinor = minorPointer(*prices.avgBalance, false)
	}
	possibleMin, possibleMax := math.Inf(1), math.Inf(-1)
	expectedMin, expectedMax := float64(0), float64(0)
	for _, prize := range draw.Prizes {
		low, high, coupon, quoteErr := s.drawPrizeExpense(ctx, prices, prize.Reward)
		if quoteErr == ErrConflict && prize.Reward.Kind == activity.RewardBalanceMultiplier {
			result.MultiplierUnavailable = true
			return result, nil
		}
		if quoteErr != nil {
			return result, quoteErr
		}
		result.CouponOneTermOnly = result.CouponOneTermOnly || coupon
		if draw.Kind == "instant" {
			possibleMin = math.Min(possibleMin, low)
			possibleMax = math.Max(possibleMax, high)
			expectedMin += float64(prize.ProbabilityBPS) / 10000 * low * float64(entries)
			expectedMax += float64(prize.ProbabilityBPS) / 10000 * high * float64(entries)
		} else {
			expectedMin += float64(prize.Stock) * low
			expectedMax += float64(prize.Stock) * high
		}
	}
	if draw.Kind == "instant" {
		possibleMin *= float64(entries)
		possibleMax *= float64(entries)
	} else {
		possibleMin, possibleMax = expectedMin, expectedMax
	}
	result.ExpectedExpenseMinMinor = minorPointer(expectedMin, false)
	result.ExpectedExpenseMaxMinor = minorPointer(expectedMax, true)
	result.PossibleExpenseMinMinor = minorPointer(possibleMin, false)
	result.PossibleExpenseMaxMinor = minorPointer(possibleMax, true)
	result.BreakEvenMinMinor = breakEvenPointer(possibleMin, entries)
	result.BreakEvenMaxMinor = breakEvenPointer(possibleMax, entries)
	return result, nil
}
