package database

import (
	"math/big"
	"time"
)

// HourlyPrefetchWindow is the bounded lead time before a paid boundary in which
// a settlement may prepay the next whole UTC hour. Extending paid authority
// before the old boundary is what removes the hourly access interruption; the
// window stays short so a member is never more than the current hour plus one
// settled hour ahead, and the debit still uses the same short-lived readiness
// proof. A boundary that already passed keeps the existing current-remainder
// recovery instead.
const HourlyPrefetchWindow = time.Minute

// hourlyPrefetchLead is the effective admission window. Hosted tests widen it to
// exercise the prefetch path deterministically because billing time is the
// PostgreSQL clock; production never assigns it.
var hourlyPrefetchLead = HourlyPrefetchWindow

// hourlyCharge rounds one deliverable part of a UTC hour upward in TXB minor
// units. Integer arithmetic keeps both subsecond activation and large prices
// exact; callers never charge an interval before access can be made available.
func hourlyCharge(price int64, from, until time.Time) (int64, error) {
	if price < 0 || !until.After(from) || until.Sub(from) > time.Hour {
		return 0, ErrConflict
	}
	numerator := new(big.Int).Mul(big.NewInt(price), big.NewInt(int64(until.Sub(from))))
	denominator := big.NewInt(int64(time.Hour))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if remainder.Sign() != 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, ErrConflict
	}
	return quotient.Int64(), nil
}

func hourlyBoundary(at time.Time) time.Time { return at.UTC().Truncate(time.Hour).Add(time.Hour) }

// hourlyPrefetchStart returns the whole-hour interval start a near-boundary
// settlement may prepay. Only an hour-aligned future paid boundary inside the
// bounded lead window qualifies, so activation's prorated remainder is never
// mistaken for an aligned hour and a duplicate worker cannot buy a second hour.
func hourlyPrefetchStart(paidThrough, now time.Time) (time.Time, bool) {
	start := paidThrough.UTC()
	if !start.After(now) || start.After(now.Add(hourlyPrefetchLead)) || !start.Equal(start.Truncate(time.Hour)) {
		return time.Time{}, false
	}
	return start, true
}
