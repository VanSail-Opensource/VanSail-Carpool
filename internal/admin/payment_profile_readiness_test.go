package admin

import (
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestPaymentReadinessChecksEveryEnabledAccount(t *testing.T) {
	t.Parallel()
	for _, first := range []model.PaymentProfile{
		{ID: "disabled", Provider: "ezpay"},
		{ID: "ready", Provider: "ezpay", Enabled: true, Configured: true, MerchantID: "merchant", EnabledChannels: []string{"alipay"}},
	} {
		t.Run(first.ID, func(t *testing.T) {
			t.Parallel()
			repository := &paymentProfileRepositoryStub{profiles: []model.PaymentProfile{
				first,
				{ID: "broken-one", Provider: "ezpay", Enabled: true},
				{ID: "broken-two", Provider: "ezpay", Enabled: true},
			}}
			service := NewSettingsService(newAdminSettingsRepository(), testVault(t))
			service.SetPaymentProfileRepository(repository)
			issues, found := service.paymentProfileReadiness(t.Context(), "ezpay")
			if !found || len(issues) != 2 || issues[0] != "missing:payment_profile.ezpay" || issues[1] != "missing:payment_profile.ezpay.merchant_id" {
				t.Fatal("later enabled accounts were ignored or issues were duplicated", issues, found)
			}
		})
	}
}
