package billing

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestBEPusdtCallbackProfileUsesOrderAccount(t *testing.T) {
	t.Parallel()

	repository := newBillingRepository()
	repository.orders["order-1"] = model.PaymentOrder{
		ID: "order-1", Provider: "bepusdt", MethodID: "bepusdt:account-one:usdt.trc20",
	}
	settings := &billingProfileRuntimeSettings{
		billingSettings: &billingSettings{},
		profile: model.PaymentProfileRuntime{
			PaymentProfile:      model.PaymentProfile{ID: "account-one", Provider: "bepusdt"},
			CredentialPlaintext: "account-secret",
		},
	}
	service := newBillingServiceForTest(repository, settings, &billingGateway{})

	profileID, valid := service.BEPusdtCallbackProfile(context.Background(), "order-1", callbackCapability("account-secret", "order-1"))
	if !valid || profileID != "account-one" {
		t.Fatalf("BEPusdtCallbackProfile() = %q, %t", profileID, valid)
	}
}

type billingProfileRuntimeSettings struct {
	*billingSettings
	profile model.PaymentProfileRuntime
}

func (s *billingProfileRuntimeSettings) PaymentProfileByID(context.Context, string, string) (model.PaymentProfileRuntime, error) {
	return s.profile, nil
}

func TestLegacyBEPusdtCallbackRejectsEmptyCredential(t *testing.T) {
	t.Parallel()
	repository := newBillingRepository()
	repository.orders["legacy-order"] = model.PaymentOrder{
		ID: "legacy-order", Provider: "bepusdt", MethodID: "bepusdt:usdt.trc20",
	}
	settings := &billingSettings{values: map[string]string{"billing.bepusdt.api_token": ""}}
	service := newBillingServiceForTest(repository, settings, &billingGateway{})
	if _, valid := service.BEPusdtCallbackProfile(context.Background(), "legacy-order", callbackCapability("", "legacy-order")); valid {
		t.Fatal("an absent historical credential accepted a public empty-key HMAC")
	}
	if _, err := service.providerCredential(context.Background(), "bepusdt", ""); err == nil {
		t.Fatal("empty legacy credential was accepted for checkout creation")
	}
	settings.values["billing.bepusdt.api_token"] = "retained-secret"
	if _, valid := service.BEPusdtCallbackProfile(context.Background(), "legacy-order", callbackCapability("retained-secret", "legacy-order")); !valid {
		t.Fatal("retained historical credential no longer verifies callbacks")
	}
}
