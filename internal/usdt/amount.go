package usdt

import (
	"math/big"
	"strings"
)

// Atomic parses fixed decimals exactly without floating point or int64 limits.
func Atomic(value string, decimals int) (*big.Int, error) {
	if decimals < 0 || decimals > 18 || len(value) > 80 {
		return nil, ErrConfiguration
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return nil, ErrConfiguration
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if fraction == "" {
			return nil, ErrConfiguration
		}
	}
	if len(fraction) > decimals {
		return nil, ErrConfiguration
	}
	digits := parts[0] + fraction + strings.Repeat("0", decimals-len(fraction))
	for _, ch := range digits {
		if ch < '0' || ch > '9' {
			return nil, ErrConfiguration
		}
	}
	result, ok := new(big.Int).SetString(digits, 10)
	if !ok || result.Sign() <= 0 {
		return nil, ErrConfiguration
	}
	return result, nil
}

// FormatAtomic renders a non-negative integer with exact fixed precision.
func FormatAtomic(value *big.Int, decimals int) string {
	digits := value.String()
	if decimals == 0 {
		return digits
	}
	if len(digits) <= decimals {
		digits = strings.Repeat("0", decimals-len(digits)+1) + digits
	}
	point := len(digits) - decimals
	return digits[:point] + "." + digits[point:]
}

// TransferAmount validates canonical positive atomic units and renders their value.
func TransferAmount(raw string, decimals int) (string, error) {
	if len(raw) > 80 {
		return "", ErrProof
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return "", ErrProof
		}
	}
	value, ok := new(big.Int).SetString(raw, 10)
	if !ok || value.Sign() <= 0 || value.String() != raw {
		return "", ErrProof
	}
	return FormatAtomic(value, decimals), nil
}
