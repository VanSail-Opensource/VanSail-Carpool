package billing

import (
	"errors"
	"fmt"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

// ParseMethodSelection validates a payment method identifier and returns its
// provider, optional profile, and provider-owned rail. The two-part form is
// retained for legacy orders; new profile-backed methods use
// provider:profileID:rail so multiple provider accounts can coexist.
func ParseMethodSelection(methodID string) (provider, profileID, rail string, err error) {
	methodID = strings.ToLower(strings.TrimSpace(methodID))
	// Internal compatibility aliases keep older workers/tests readable. The HTTP
	// transport accepts only canonical IDs and never exposes these aliases.
	if methodID == "ezpay" {
		return "ezpay", "", "alipay", nil
	}
	if methodID == "bepusdt" {
		return "bepusdt", "", "usdt.trc20", nil
	}
	if methodID == "stars" {
		return "stars", "", "", nil
	}
	if methodID == "coupon" {
		return "coupon", "", "", nil
	}
	parts := strings.Split(methodID, ":")
	if (len(parts) != 2 && len(parts) != 3) || parts[0] == "" || parts[len(parts)-1] == "" || (len(parts) == 3 && parts[1] == "") {
		return "", "", "", fmt.Errorf("%w: malformed method id", ErrInvalidOrder)
	}
	profileID = ""
	railIndex := 1
	if len(parts) == 3 {
		profileID = parts[1]
		railIndex = 2
	}
	rail = parts[railIndex]
	switch parts[0] {
	case "builtin_usdt":
		if len(parts) != 2 {
			return "", "", "", ErrInvalidOrder
		}
		if spec, ok := usdt.Network(rail); !ok || spec.Rail != rail {
			return "", "", "", ErrInvalidOrder
		}
	case "ezpay":
		if _, ok := ezpayRails[rail]; !ok {
			return "", "", "", fmt.Errorf("%w: unsupported EZPay rail", ErrInvalidOrder)
		}
	case "bepusdt":
		if _, ok := bepusdtRails[rail]; !ok {
			return "", "", "", fmt.Errorf("%w: unsupported BEPusdt rail", ErrInvalidOrder)
		}
	default:
		return "", "", "", fmt.Errorf("%w: unsupported provider", ErrInvalidOrder)
	}
	return parts[0], profileID, rail, nil
}

// ParseMethodID validates a stable payment method identifier and returns its
// provider-owned rail. Stars intentionally has no sub-rail.
func ParseMethodID(methodID string) (provider, rail string, err error) {
	provider, _, rail, err = ParseMethodSelection(methodID)
	return provider, rail, err
}

// CanonicalMethodID reports whether the public identifier is stable and fully
// qualifies its provider rail.
func CanonicalMethodID(methodID string) bool {
	methodID = strings.ToLower(strings.TrimSpace(methodID))
	if methodID == "stars" || methodID == "coupon" {
		return true
	}
	_, _, _, err := ParseMethodSelection(methodID)
	return err == nil && (strings.HasPrefix(methodID, "ezpay:") || strings.HasPrefix(methodID, "bepusdt:") || strings.HasPrefix(methodID, "builtin_usdt:"))
}

func parseEnabledRails(raw string, allowed map[string]string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("at least one rail is required")
	}
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, value := range strings.Split(raw, ",") {
		value = strings.ToLower(strings.TrimSpace(value))
		if _, ok := allowed[value]; !ok {
			return nil, fmt.Errorf("unsupported rail %q", value)
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, fmt.Errorf("duplicate rail %q", value)
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func methodName(provider, rail string) string {
	if provider == "coupon" {
		return "Coupon"
	}
	if provider == "stars" {
		return "Telegram Stars"
	}
	if provider == "ezpay" {
		return ezpayRails[rail]
	}
	return bepusdtRails[rail]
}

func methodModel(provider, profileID, rail string, available bool, note string) model.PaymentMethod {
	id := provider
	if rail != "" {
		if profileID != "" {
			id += ":" + profileID
		}
		id += ":" + rail
	}
	return model.PaymentMethod{
		ID: id, Provider: provider, ProfileID: profileID, Rail: rail, Name: methodName(provider, rail),
		Currency: strings.ToUpper(currencyCode(provider)), Available: available, Note: note, Mode: "order",
	}
}

func containsRail(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func validateEnabledRails(value string, allowed map[string]string) error {
	_, err := parseEnabledRails(value, allowed)
	return err
}

// ValidateEZPayMethods validates an ordered comma-separated EZPay rail list.
func ValidateEZPayMethods(value string) error { return validateEnabledRails(value, ezpayRails) }

// ValidateBEPusdtMethods validates an ordered comma-separated USDT rail list.
func ValidateBEPusdtMethods(value string) error { return validateEnabledRails(value, bepusdtRails) }

var errRateNotConfigured = errors.New("payment rate is not configured")
