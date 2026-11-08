package model

import (
	"errors"
	"math"
	"time"
)

var ErrHourlyPolicyInvalid = errors.New("invalid hourly policy")

// ValidateHourlyPolicy validates the same exact units for administration and
// wallet transactions. Unconfigured defaults cannot authorize a sale.
func ValidateHourlyPolicy(policy HourlyPolicy) error {
	if policy.DailyAllowanceBytes < 0 || policy.ReducedRateBytesPerSecond > 1_000_000_000_000 || policy.SalesEnabled && policy.ReducedRateBytesPerSecond == 0 || policy.ExtraGBPriceTXBMinor < 0 || policy.ActivationHours <= 0 || policy.ActivationHours > int64(time.Duration(math.MaxInt64)/time.Hour) || policy.CooldownSeconds < 0 || policy.CooldownSeconds > int64(time.Duration(math.MaxInt64)/time.Second) {
		return ErrHourlyPolicyInvalid
	}
	return nil
}

// HourlyActivationMinimum is an eligibility check, never a debit/reservation.
func HourlyActivationMinimum(policy HourlyPolicy, hourlyPriceMinor int64) (int64, error) {
	if ValidateHourlyPolicy(policy) != nil || hourlyPriceMinor <= 0 || hourlyPriceMinor > math.MaxInt64/policy.ActivationHours {
		return 0, ErrHourlyPolicyInvalid
	}
	return hourlyPriceMinor * policy.ActivationHours, nil
}
