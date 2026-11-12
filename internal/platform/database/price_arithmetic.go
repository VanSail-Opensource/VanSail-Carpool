package database

import (
	"fmt"
	"math"
)

// Commerce sums must be checked before Go integer arithmetic can wrap. A
// PostgreSQL nonnegative constraint cannot detect a positive wrapped total.
func checkedPriceSum(left, right int64) (int64, error) {
	if left < 0 || right < 0 || left > math.MaxInt64-right {
		return 0, fmt.Errorf("%w: price exceeds integer range", ErrConflict)
	}
	return left + right, nil
}

func checkedPriceTotal(price int64, terms int) (int64, error) {
	if price < 0 || terms < 1 || price > math.MaxInt64/int64(terms) {
		return 0, fmt.Errorf("%w: total price exceeds integer range", ErrConflict)
	}
	return price * int64(terms), nil
}
