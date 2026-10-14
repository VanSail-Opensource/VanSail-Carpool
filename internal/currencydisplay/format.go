// Package currencydisplay converts display-only TXB values using fixed decimal rates.
package currencydisplay

import (
	"errors"
	"math/big"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Currency identifies a supported member-facing monetary display.
type Currency string

const (
	TXB Currency = "TXB"
	CNY Currency = "CNY"
	USD Currency = "USD"
)

// Rates contains the configured TXB-per-currency conversion rates.
type Rates struct {
	CNYTXBPerUnit string
	USDTXBPerUnit string
}

// ParseCurrency normalizes a supported display currency.
func ParseCurrency(value string) (Currency, bool) {
	switch Currency(strings.ToUpper(strings.TrimSpace(value))) {
	case TXB:
		return TXB, true
	case CNY:
		return CNY, true
	case USD:
		return USD, true
	default:
		return "", false
	}
}

// ValidRate reports whether a fixed-decimal TXB-per-currency rate is usable.
func ValidRate(value string) bool {
	_, _, err := parseRate(value)
	return err == nil
}

// FormatMoney converts a TXB amount into target using a TXB-per-currency rate.
// The target value is truncated toward zero to two decimal places.
func FormatMoney(money model.Money, target Currency, rate string) (model.Money, error) {
	if target == TXB || strings.ToUpper(money.Currency) != string(TXB) {
		return money, nil
	}
	coefficient, scale, err := parseRate(rate)
	if err != nil {
		return model.Money{}, err
	}
	minor, ok := new(big.Int).SetString(strings.TrimSpace(money.Minor), 10)
	if !ok {
		return model.Money{}, errors.New("invalid TXB minor amount")
	}
	negative := minor.Sign() < 0
	minor.Abs(minor)
	numerator := new(big.Int).Mul(minor, powerOfTen(scale))
	targetMinor := new(big.Int).Quo(numerator, coefficient)
	if negative {
		targetMinor.Neg(targetMinor)
	}
	return model.Money{
		Currency: string(target),
		Minor:    targetMinor.String(),
		Display:  formatMinor(targetMinor, target),
	}, nil
}

// FormatMoneyWithFallback converts a member-facing amount without displaying a
// zero-valued conversion for a nonzero TXB amount. USD falls back to CNY when
// available, and CNY or an unavailable fallback ultimately returns TXB.
func FormatMoneyWithFallback(money model.Money, target Currency, rates Rates) (model.Money, error) {
	converted, err := FormatMoney(money, target, rates.rateFor(target))
	if err != nil || !isNonzeroMinor(money.Minor) || converted.Minor != "0" {
		return converted, err
	}
	if target == USD {
		cny, cnyErr := FormatMoney(money, CNY, rates.CNYTXBPerUnit)
		if cnyErr == nil && cny.Minor != "0" {
			return cny, nil
		}
	}
	return money, nil
}

func (r Rates) rateFor(target Currency) string {
	if target == USD {
		return r.USDTXBPerUnit
	}
	return r.CNYTXBPerUnit
}

func isNonzeroMinor(value string) bool {
	minor, ok := new(big.Int).SetString(strings.TrimSpace(value), 10)
	return ok && minor.Sign() != 0
}

func parseRate(value string) (*big.Int, int, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ".")
	if len(parts) > 2 || len(parts) == 0 || parts[0] == "" || len(parts) == 2 && parts[1] == "" {
		return nil, 0, errors.New("invalid display currency rate")
	}
	for _, part := range parts {
		for _, character := range part {
			if character < '0' || character > '9' {
				return nil, 0, errors.New("invalid display currency rate")
			}
		}
	}
	coefficient, ok := new(big.Int).SetString(strings.Join(parts, ""), 10)
	if !ok || coefficient.Sign() <= 0 {
		return nil, 0, errors.New("invalid display currency rate")
	}
	scale := 0
	if len(parts) == 2 {
		scale = len(parts[1])
	}
	return coefficient, scale, nil
}

func powerOfTen(exponent int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil)
}

func formatMinor(value *big.Int, currency Currency) string {
	negative := value.Sign() < 0
	abs := new(big.Int).Abs(new(big.Int).Set(value))
	whole := new(big.Int).Quo(abs, big.NewInt(100))
	fraction := new(big.Int).Mod(abs, big.NewInt(100)).Text(10)
	return sign(negative) + whole.Text(10) + "." + strings.Repeat("0", 2-len(fraction)) + fraction + " " + string(currency)
}

func sign(negative bool) string {
	if negative {
		return "-"
	}
	return ""
}
