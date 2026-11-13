package database

import (
	"errors"
	"math"
	"math/big"
	"strconv"
)

// ErrPricingSummaryNonFinite rejects estimates that cannot represent an amount.
var ErrPricingSummaryNonFinite = errors.New("pricing summary estimate is not finite")

func pricingSummaryDifference(revenue, cost string) (string, error) {
	receipts, validReceipts := new(big.Int).SetString(revenue, 10)
	payments, validPayments := new(big.Int).SetString(cost, 10)
	if !validReceipts || !validPayments {
		return "", errors.New("pricing summary aggregate is not an integer")
	}
	return new(big.Int).Sub(receipts, payments).String(), nil
}

// Pricing summaries already expose decimal strings. Preserve the finite
// estimate's rounded value without narrowing it to a signed machine integer.
func pricingSummaryEstimate(value float64) (string, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "", ErrPricingSummaryNonFinite
	}
	rounded := math.Round(value)
	if rounded == 0 {
		return "0", nil
	}
	return strconv.FormatFloat(rounded, 'f', 0, 64), nil
}
