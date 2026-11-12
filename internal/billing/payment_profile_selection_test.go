package billing

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestQueuedLegacyPaymentSelectionPinsProfileAndReplaysOriginalIntent(t *testing.T) {
	ctx := context.Background()
	store, user := newPaymentOperationStore(t, 80_021)
	settings := &billingProfileSettings{billingSettings: &billingSettings{values: map[string]string{"billing.rate.txb_per_cny": "1"}}}
	for _, id := range []string{"first", "second"} {
		profile, err := store.SavePaymentProfile(ctx, database.PaymentProfileInput{
			ID: id, Provider: "ezpay", ProviderName: id, Endpoint: "https://pay.example", MerchantID: "merchant",
			EnabledChannels: []string{"alipay"}, Enabled: true, CredentialCiphertext: "fixture-ciphertext",
		})
		if err != nil {
			t.Fatal(err)
		}
		settings.profiles = append(settings.profiles, profile)
	}
	service := newBillingServiceForTest(store, settings, &billingGateway{})
	created, err := service.QueueOrder(ctx, user, "ezpay:alipay", 250, "pinned-key")
	if err != nil {
		t.Fatal(err)
	}
	order, err := store.PaymentOrderByID(ctx, created.PaymentOrderID)
	if err != nil || order.MethodID != "ezpay:first:alipay" {
		t.Fatalf("durable selection = %+v, %v", order, err)
	}
	settings.profiles[0], settings.profiles[1] = settings.profiles[1], settings.profiles[0]
	replayed, err := service.QueueOrder(ctx, user, "ezpay:alipay", 250, "pinned-key")
	if err != nil || replayed.PaymentOrderID != created.PaymentOrderID {
		t.Fatalf("replay after changing default = %+v, %v", replayed, err)
	}
	createdNext, err := service.QueueOrder(ctx, user, "ezpay:alipay", 250, "new-key")
	if err != nil {
		t.Fatal(err)
	}
	next, err := store.PaymentOrderByID(ctx, createdNext.PaymentOrderID)
	if err != nil || next.MethodID != "ezpay:second:alipay" {
		t.Fatalf("new intent selection = %+v, %v", next, err)
	}
}

func TestExplicitPaymentSelectionCannotUseLegacyEnabledSettings(t *testing.T) {
	t.Parallel()
	service := newBillingServiceForTest(newBillingRepository(), &billingSettings{values: map[string]string{
		"billing.ezpay.enabled": "true", "billing.rate.txb_per_cny": "1",
	}}, &billingGateway{})
	if _, err := service.prepareOrder(context.Background(), model.User{ID: "user"}, "ezpay:missing:alipay", 250); !errors.Is(err, ErrProviderDisabled) {
		t.Fatalf("missing explicit account error = %v", err)
	}
	if _, err := service.providerCredential(context.Background(), "bepusdt", "missing"); err == nil {
		t.Fatal("explicit profile silently inherited a global credential")
	}
}
