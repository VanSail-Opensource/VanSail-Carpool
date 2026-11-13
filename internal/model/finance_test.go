package model

import (
	"math"
	"strconv"
	"testing"
)

func TestTXBMoneyFormatsFullSignedRange(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		minor int64
		want  string
	}{
		{math.MinInt64, "-92233720368547758.08 TXB"},
		{math.MinInt64 + 1, "-92233720368547758.07 TXB"},
		{-101, "-1.01 TXB"},
		{-1, "-0.01 TXB"},
		{0, "0.00 TXB"},
		{1, "0.01 TXB"},
		{math.MaxInt64, "92233720368547758.07 TXB"},
	} {
		t.Run(strconv.FormatInt(test.minor, 10), func(t *testing.T) {
			money := TXBMoney(test.minor)
			if money.Currency != "TXB" || money.Minor != strconv.FormatInt(test.minor, 10) || money.Display != test.want {
				t.Fatalf("TXBMoney(%d) = %+v, want display %q", test.minor, money, test.want)
			}
			if parsed, err := ParseTXBMajor(money.Display[:len(money.Display)-4]); err != nil || parsed != test.minor {
				t.Fatalf("formatted amount lost its exact value: %d, %v", parsed, err)
			}
		})
	}
}
