package model

import (
	"errors"
	"math"
	"time"
)

var ErrHourlyPolicyInvalid = errors.New("invalid hourly policy")

// MaxHourlyRequiredHours bounds one offer's minimum enable duration to the same
// duration domain as policy.ActivationHours, which also keeps the product
// with the hourly price inside int64 minor units.
const MaxHourlyRequiredHours int64 = 2562047

// ValidateHourlyPolicy validates the same exact units for administration and
// wallet transactions. Unconfigured defaults cannot authorize a sale.
func ValidateHourlyPolicy(policy HourlyPolicy) error {
	if policy.DailyAllowanceBytes < 0 || policy.ReducedRateBytesPerSecond > 1_000_000_000_000 || policy.SalesEnabled && policy.ReducedRateBytesPerSecond == 0 || policy.ExtraGBPriceTXBMinor < 0 || policy.ActivationHours <= 0 || policy.ActivationHours > int64(time.Duration(math.MaxInt64)/time.Hour) {
		return ErrHourlyPolicyInvalid
	}
	return nil
}

// HourlyActivationMinimum is the exact nonrefundable activation prepayment.
// The existing requiredHours setting now buys that many hours; zero retains
// the policy.ActivationHours inheritance used by pre-existing offers.
func HourlyActivationMinimum(policy HourlyPolicy, requiredHours, hourlyPriceMinor int64) (int64, error) {
	hours := requiredHours
	if hours == 0 {
		hours = policy.ActivationHours
	}
	if ValidateHourlyPolicy(policy) != nil || hours < 0 || hours > MaxHourlyRequiredHours ||
		hourlyPriceMinor <= 0 || hourlyPriceMinor > math.MaxInt64/hours {
		return 0, ErrHourlyPolicyInvalid
	}
	return hourlyPriceMinor * hours, nil
}
