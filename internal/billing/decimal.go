// Package billing implements fixed-decimal pricing and authoritative payment settlement.
package billing

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ParseTXBMajor delegates to the shared exact TXB parser.
func ParseTXBMajor(raw string) (int64, error) { return model.ParseTXBMajor(raw) }

// Decimal is a non-negative base-10 value represented as integer coefficient and scale.
type Decimal struct {
	coefficient *big.Int
	scale       int
}

// ParseDecimal accepts canonical, non-negative fixed decimals without exponent notation.
func ParseDecimal(raw string) (Decimal, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 64 {
		return Decimal{}, errors.New("decimal is too long")
	}
	if raw == "" || strings.HasPrefix(raw, "-") || strings.HasPrefix(raw, "+") || strings.ContainsAny(raw, "eE") {
		return Decimal{}, errors.New("value must be a non-negative fixed decimal")
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
		return Decimal{}, errors.New("invalid decimal syntax")
	}
	digits := strings.Join(parts, "")
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return Decimal{}, errors.New("invalid decimal digit")
		}
	}
	coefficient := new(big.Int)
	if _, ok := coefficient.SetString(digits, 10); !ok {
		return Decimal{}, errors.New("invalid decimal")
	}
	scale := 0
	if len(parts) == 2 {
		scale = len(parts[1])
	}
	return Decimal{coefficient: coefficient, scale: scale}, nil
}

// Positive reports whether the decimal is greater than zero.
func (d Decimal) Positive() bool {
	return d.coefficient != nil && d.coefficient.Sign() > 0
}

// Canonical removes redundant zeroes while preserving exact value.
func (d Decimal) Canonical() string {
	if d.coefficient == nil {
		return "0"
	}
	digits := d.coefficient.String()
	if d.scale == 0 {
		return digits
	}
	if len(digits) <= d.scale {
		digits = strings.Repeat("0", d.scale-len(digits)+1) + digits
	}
	point := len(digits) - d.scale
	value := strings.TrimRight(digits[:point]+"."+digits[point:], "0")
	return strings.TrimSuffix(value, ".")
}

// Payable multiplies an integer hundredths-of-TXB request by a per-TXB rate and rounds upward.
func Payable(txbMinor int64, rate Decimal, providerPrecision int) (string, error) {
	if txbMinor <= 0 || !rate.Positive() {
		return "", errors.New("positive TXB amount and rate are required")
	}
	if providerPrecision < 0 || providerPrecision > 12 {
		return "", errors.New("provider precision is out of range")
	}
	numerator := new(big.Int).Mul(big.NewInt(txbMinor), rate.coefficient)
	numerator.Mul(numerator, pow10(providerPrecision))
	denominator := new(big.Int).Mul(big.NewInt(100), pow10(rate.scale))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if remainder.Sign() > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return formatScaled(quotient, providerPrecision), nil
}

// PayableFromTXBPerCurrency divides the requested TXB by a TXB-per-currency
// rate and rounds the provider amount upward. This is the authoritative rate
// direction for newly created payment orders.
func PayableFromTXBPerCurrency(txbMinor int64, rate Decimal, providerPrecision int) (string, error) {
	if txbMinor <= 0 || !rate.Positive() {
		return "", errors.New("positive TXB amount and rate are required")
	}
	if providerPrecision < 0 || providerPrecision > 12 {
		return "", errors.New("provider precision is out of range")
	}
	numerator := new(big.Int).Mul(big.NewInt(txbMinor), pow10(rate.scale+providerPrecision))
	denominator := new(big.Int).Mul(big.NewInt(100), rate.coefficient)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if remainder.Sign() > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return formatScaled(quotient, providerPrecision), nil
}

// Equivalent compares two decimal lexical forms by value.
func Equivalent(left, right string) bool {
	a, err := ParseDecimal(left)
	if err != nil {
		return false
	}
	b, err := ParseDecimal(right)
	if err != nil {
		return false
	}
	scale := max(a.scale, b.scale)
	ac := new(big.Int).Mul(a.coefficient, pow10(scale-a.scale))
	bc := new(big.Int).Mul(b.coefficient, pow10(scale-b.scale))
	return ac.Cmp(bc) == 0
}

func formatScaled(value *big.Int, scale int) string {
	digits := value.String()
	if scale == 0 {
		return digits
	}
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale-len(digits)+1) + digits
	}
	point := len(digits) - scale
	return fmt.Sprintf("%s.%s", digits[:point], digits[point:])
}

func pow10(exponent int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil)
}
