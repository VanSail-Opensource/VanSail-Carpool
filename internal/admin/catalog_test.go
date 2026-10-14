package admin

import (
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
)

func TestSettingValidatorsCoverOptionalFormats(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		fn   func(string) error
		good string
		bad  string
	}{
		{"nonnegative TXB", validateNonnegativeTXB, "0.01", "-1"},
		{"timezone", validateTimezone, "UTC", "Not/ATimezone"},
		{"nonnegative integer", validateNonnegativeInteger, "0", "-1"},
		{"webhook secret", validateWebhookSecret, "safe_secret-1", "contains space"},
		{"BEPusdt methods", billing.ValidateBEPusdtMethods, "usdt.trc20,usdt.ton", "usdc"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.fn(test.good); err != nil {
				t.Errorf("valid value: %v", err)
			}
			if err := test.fn(test.bad); err == nil {
				t.Errorf("invalid value %q was accepted", test.bad)
			}
		})
	}
}
