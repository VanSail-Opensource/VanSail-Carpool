package database

import (
	"math/big"
	"time"
)

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
