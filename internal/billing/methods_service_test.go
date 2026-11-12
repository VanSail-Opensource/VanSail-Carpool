package billing

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestMethodsReturnsOrderedAvailability(t *testing.T) {
	t.Parallel()
	settings := &billingSettings{values: map[string]string{
		"billing.ezpay.enabled":    "true",
		"billing.ezpay.methods":    "wxpay,alipay",
		"billing.rate.txb_per_cny": "10",
		"billing.bepusdt.enabled":  "true",
		"billing.bepusdt.methods":  "usdt.trc20",
		"billing.stars.enabled":    "true",
		"billing.rate.txb_per_xtr": "2",
	}}
	methods, err := newBillingServiceForTest(newBillingRepository(), settings, &billingGateway{}).Methods(context.Background())
	if err != nil {
		t.Fatalf("Methods(): %v", err)
	}
	if len(methods) != 5 || methods[0].ID != "coupon" || methods[0].Mode != "coupon_redemption" || methods[1].ID != "ezpay:wxpay" || methods[2].ID != "ezpay:alipay" || methods[3].Available || !methods[4].Available {
		t.Fatalf("Methods() = %+v", methods)
	}
	if methods[3].Note == "" || methods[4].Name != "Telegram Stars" || methods[4].Currency != "XTR" {
		t.Fatalf("method metadata = %+v", methods)
	}
}

func TestMethodsReturnsEveryConfiguredProviderAccount(t *testing.T) {
	t.Parallel()

	settings := &billingProfileSettings{
		billingSettings: &billingSettings{values: map[string]string{"billing.rate.txb_per_cny": "10"}},
		profiles: []model.PaymentProfile{
			{ID: "ezpay-main", Provider: "ezpay", ProviderName: "Main EZPay", EnabledChannels: []string{"alipay"}, Enabled: true, Configured: true},
			{ID: "ezpay-backup", Provider: "ezpay", ProviderName: "Backup EZPay", EnabledChannels: []string{"wxpay"}, Enabled: true, Configured: true},
		},
	}
	methods, err := newBillingServiceForTest(newBillingRepository(), settings, &billingGateway{}).Methods(context.Background())
	if err != nil {
		t.Fatalf("Methods(): %v", err)
	}
	if len(methods) != 3 || methods[1].ID != "ezpay:ezpay-main:alipay" || methods[1].ProviderName != "Main EZPay" || methods[2].ID != "ezpay:ezpay-backup:wxpay" || methods[2].ProviderName != "Backup EZPay" {
		t.Fatalf("Methods() = %+v", methods)
	}
}

type billingProfileSettings struct {
	*billingSettings
	profiles []model.PaymentProfile
}

func (s *billingProfileSettings) PaymentProfiles(context.Context) ([]model.PaymentProfile, error) {
	return s.profiles, nil
}

func TestMethodsRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()
	settings := &billingSettings{values: map[string]string{
		"billing.ezpay.enabled":    "true",
		"billing.ezpay.methods":    "not-a-rail",
		"billing.rate.txb_per_cny": "1",
	}}
	if _, err := newBillingServiceForTest(newBillingRepository(), settings, &billingGateway{}).Methods(context.Background()); err == nil {
		t.Fatal("Methods() accepted an invalid rail")
	}
	settings.errs = map[string]error{"billing.ezpay.enabled": errors.New("settings unavailable")}
	if _, err := newBillingServiceForTest(newBillingRepository(), settings, &billingGateway{}).Methods(context.Background()); err == nil {
		t.Fatal("Methods() ignored settings failure")
	}
}

func TestVerifyBEPusdtCallbackCapability(t *testing.T) {
	t.Parallel()
	service := newBillingServiceForTest(newBillingRepository(), &billingSettings{values: map[string]string{"billing.bepusdt.api_token": "secret"}}, &billingGateway{})
	valid := callbackCapability("secret", "order-1")
	if !service.VerifyBEPusdtCallbackCapability(context.Background(), "order-1", valid) {
		t.Fatal("VerifyBEPusdtCallbackCapability() rejected valid capability")
	}
	if service.VerifyBEPusdtCallbackCapability(context.Background(), "order-1", "wrong") || service.VerifyBEPusdtCallbackCapability(context.Background(), "order-2", valid) {
		t.Fatal("VerifyBEPusdtCallbackCapability() accepted invalid capability")
	}
	service.settings = &billingSettings{errs: map[string]error{"billing.bepusdt.api_token": errors.New("secret unavailable")}}
	if service.VerifyBEPusdtCallbackCapability(context.Background(), "order-1", valid) {
		t.Fatal("VerifyBEPusdtCallbackCapability() accepted unavailable secret")
	}
}
