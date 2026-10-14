package currencydisplay

import (
	"strconv"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestFormatMoneyUsesFixedPointFlooring(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, minor, rate, want string
		target                  Currency
	}{
		{name: "CNY exact", minor: "1250", rate: "10", target: CNY, want: "1.25 CNY"},
		{name: "USD fractional floor", minor: "100", rate: "3", target: USD, want: "0.33 USD"},
		{name: "negative truncates toward zero", minor: "-100", rate: "3", target: USD, want: "-0.33 USD"},
		{name: "decimal rate", minor: "1250", rate: "2.5", target: CNY, want: "5.00 CNY"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			money, err := FormatMoney(model.TXBMoney(mustMinor(t, test.minor)), test.target, test.rate)
			if err != nil || money.Display != test.want {
				t.Fatalf("FormatMoney() = (%q, %v), want (%q, nil)", money.Display, err, test.want)
			}
		})
	}
}

func TestFormatMoneyRejectsInvalidRate(t *testing.T) {
	t.Parallel()
	if _, err := FormatMoney(model.TXBMoney(100), CNY, "0"); err == nil {
		t.Fatal("FormatMoney() accepted a zero rate")
	}
}

func TestFormatMoneyWithFallback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		minor        int64
		target       Currency
		rates        Rates
		wantCurrency string
		wantDisplay  string
	}{
		{name: "nonzero USD falls back to CNY", minor: 1, target: USD, rates: Rates{CNYTXBPerUnit: "0.2", USDTXBPerUnit: "1000"}, wantCurrency: "CNY", wantDisplay: "0.05 CNY"},
		{name: "nonzero CNY falls back to TXB", minor: 1, target: CNY, rates: Rates{CNYTXBPerUnit: "1000"}, wantCurrency: "TXB", wantDisplay: "0.01 TXB"},
		{name: "zero TXB remains zero USD", minor: 0, target: USD, rates: Rates{CNYTXBPerUnit: "0.2", USDTXBPerUnit: "1000"}, wantCurrency: "USD", wantDisplay: "0.00 USD"},
		{name: "missing CNY fallback returns TXB", minor: 1, target: USD, rates: Rates{USDTXBPerUnit: "1000"}, wantCurrency: "TXB", wantDisplay: "0.01 TXB"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			money, err := FormatMoneyWithFallback(model.TXBMoney(test.minor), test.target, test.rates)
			if err != nil || money.Currency != test.wantCurrency || money.Display != test.wantDisplay {
				t.Fatalf("FormatMoneyWithFallback() = (%+v, %v), want (%s, %s, nil)", money, err, test.wantCurrency, test.wantDisplay)
			}
		})
	}
}

func TestValidRate(t *testing.T) {
	t.Parallel()
	if !ValidRate("2.50") || ValidRate("0") || ValidRate("invalid") {
		t.Fatal("ValidRate() accepted an invalid fixed-decimal rate")
	}
}

func mustMinor(t *testing.T, value string) int64 {
	t.Helper()
	result, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
