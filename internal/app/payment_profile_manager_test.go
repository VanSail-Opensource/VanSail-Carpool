package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestPaymentProfileFailureMessageUsesLocaleAndEscapesValues(t *testing.T) {
	profile := model.PaymentProfile{ProviderName: "Provider_[1]"}
	for _, test := range []struct{ locale, title, label string }{
		{"en", "BEPUSDT payment profile disabled", "*Reason:*"},
		{"zh-CN", "BEPUSDT 支付配置已停用", "*原因:*"},
	} {
		message := paymentProfileFailureMessage(profile, errors.New("bad_[token]"), test.locale)
		for _, want := range []string{test.title, test.label, `Provider\_\[1\]`, `bad\_\[token\]`} {
			if !strings.Contains(message, want) {
				t.Errorf("%s missing %q in %q", test.locale, want, message)
			}
		}
	}
}
