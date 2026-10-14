package admin

import (
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func validatePositiveDecimal(value string) error {
	decimal, err := billing.ParseDecimal(value)
	if err != nil || !decimal.Positive() {
		return errors.New("must be a positive fixed decimal")
	}
	return nil
}

func validateNonnegativeTXB(value string) error {
	minor, err := billing.ParseTXBMajor(value)
	if err != nil || minor < 0 {
		return errors.New("must be a non-negative TXB amount with at most two decimals")
	}
	return nil
}

func validateTimezone(value string) error {
	if _, err := time.LoadLocation(strings.TrimSpace(value)); err != nil {
		return errors.New("must be an IANA timezone")
	}
	return nil
}

func validateInteger(value string) error {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed == 0 {
		return errors.New("must be a non-zero integer")
	}
	return nil
}

func validateOptionalInteger(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return validateInteger(value)
}

func validateNonnegativeInteger(value string) error {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return errors.New("must be a non-negative integer")
	}
	return nil
}

func validateHTTPSURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("must be an absolute HTTPS URL without credentials, query, or fragment")
	}
	return nil
}

func validateHTTPOrHTTPSURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("must be an absolute HTTP or HTTPS URL without credentials, query, or fragment")
	}
	return nil
}

func validateBoolean(value string) error {
	if value != "true" && value != "false" {
		return errors.New("must be true or false")
	}
	return nil
}

func validateAck(value string) error {
	if value != "ok" && !strings.EqualFold(value, "success") {
		return errors.New("must be ok or success")
	}
	return nil
}

// Kept for compatibility with callers validating one legacy selection. New
// configuration uses the ordered billing.ezpay.methods list.
func validateEZPayType(value string) error {
	return billing.ValidateEZPayMethods(value)
}

var webhookSecretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

func validateWebhookSecret(value string) error {
	if !webhookSecretPattern.MatchString(value) {
		return errors.New("must contain 1-256 URL-safe characters")
	}
	return nil
}

func nonempty(value string) error {
	if value == "" {
		return errors.New("must not be empty")
	}
	return nil
}
