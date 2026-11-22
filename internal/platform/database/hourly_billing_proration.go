package database

import (
	"math/big"
	"time"
)

// HourlyPrefetchWindow is the bounded lead time before a paid boundary in which
// a settlement may prepay the next deliverable UTC-hour interval. Extending
// paid authority before the old boundary prevents an hourly access interruption,
// and the debit still uses the same short-lived readiness proof. A boundary that
// already passed keeps the existing current-remainder recovery instead.
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

// Mirror hourlyCharge for SQL admission scans: use exact numeric arithmetic
// before multiplying large prices, and always truncate the boundary in UTC.
// At an hour-aligned boundary this is the whole hourly price.
const hourlyRenewalPriceSQL = `div(o.hourly_price_minor::numeric * EXTRACT(EPOCH FROM (
 (date_trunc('hour', e.paid_through AT TIME ZONE 'UTC') + INTERVAL '1 hour') -
 (e.paid_through AT TIME ZONE 'UTC'))) * 1000000 + 3599999999, 3600000000)`

// hourlyPrefetchStart returns the paid boundary a near-boundary settlement may
// extend. A minimum enable interval ends at activation's offset within the UTC
// hour; its first renewal buys only the remaining fraction after that boundary.
// Later renewals return to whole UTC hours. No prepaid time is charged again.
func hourlyPrefetchStart(paidThrough, now time.Time) (time.Time, bool) {
	start := paidThrough.UTC()
	if !start.After(now) || start.After(now.Add(hourlyPrefetchLead)) {
		return time.Time{}, false
	}
	return start, true
}
