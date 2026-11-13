package bepusdt

import (
	"errors"
	"math/big"
	"net/url"
	"regexp"
)

var decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
var responseDecimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]{1,2})?$`)

func validateCallbackURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		return errors.New("URL must be absolute http(s) without credentials")
	}
	return nil
}

func validatePositiveDecimal(value string) error {
	if err := validateDecimal(value); err != nil {
		return err
	}
	parsed, _ := new(big.Rat).SetString(value)
	if parsed.Sign() <= 0 {
		return errors.New("value must be positive")
	}
	return nil
}

func validateDecimal(value string) error {
	if len(value) > 64 {
		return errors.New("value is too long")
	}
	if !decimalPattern.MatchString(value) {
		return errors.New("value must be a base-10 decimal")
	}
	if _, ok := new(big.Rat).SetString(value); !ok {
		return errors.New("value is outside supported decimal syntax")
	}
	return nil
}

func decimalEquivalent(left, right string) bool {
	// Provider response fields are untrusted. Bound both operands before big.Rat
	// can expand a compact exponent or allocate an arbitrarily large integer.
	if len(left) > 64 || len(right) > 64 || !responseDecimalPattern.MatchString(left) || !responseDecimalPattern.MatchString(right) {
		return false
	}
	leftValue, leftOK := new(big.Rat).SetString(left)
	rightValue, rightOK := new(big.Rat).SetString(right)
	return leftOK && rightOK && leftValue.Cmp(rightValue) == 0
}
