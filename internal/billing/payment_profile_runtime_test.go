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
