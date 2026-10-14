package purchaseops

import (
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestResetPriceMinorRoundsUp(t *testing.T) {
	tests := []struct {
		name, strategy string
		gross, want    int64
		valid          bool
	}{
		{name: "daily exact", strategy: "DAY", gross: 300, want: 10, valid: true},
		{name: "daily rounds up", strategy: "DAY", gross: 301, want: 11, valid: true},
		{name: "weekly rounds up", strategy: "WEEK", gross: 101, want: 26, valid: true},
		{name: "rolling month full", strategy: "MONTH_ROLLING", gross: 101, want: 101, valid: true},
		{name: "legacy month rejected", strategy: "MONTH", gross: 101},
		{name: "zero rejected", strategy: "DAY", gross: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := ResetPriceMinor(test.gross, test.strategy)
			if got != test.want || valid != test.valid {
				t.Fatalf("ResetPriceMinor() = (%d, %t), want (%d, %t)", got, valid, test.want, test.valid)
			}
		})
	}
}

func TestQuoteMemberRefundUsesStrictWindowAndUsage(t *testing.T) {
	created := time.Date(2026, 8, 18, 1, 0, 0, 0, time.UTC)
	facts := PurchaseFacts{FirstTerm: true, Purchase: model.Purchase{ID: "purchase", UserID: "user", Status: "active", PriceTXBMinor: 245,
		ValidFrom: created, ValidUntil: created.Add(30 * 24 * time.Hour), CreatedAt: created}}
	tests := []struct {
		name string
		now  time.Time
		used int64
		want bool
	}{
		{name: "inside window", now: created.Add(24*time.Hour - time.Nanosecond), want: true},
		{name: "at boundary", now: created.Add(24 * time.Hour)},
		{name: "traffic used", now: created.Add(time.Hour), used: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			quote, err := QuoteMemberRefund(facts, "user", test.used, test.now)
			if err != nil || quote.Eligible != test.want {
				t.Fatalf("QuoteMemberRefund() = (%+v, %v), want eligible=%t", quote, err, test.want)
			}
		})
	}
}
