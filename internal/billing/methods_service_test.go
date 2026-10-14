package billing

import (
	"context"
	"errors"
	"testing"
	"time"

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

type billingCancellationRepository struct {
	*billingRepository
	changed             bool
	cancelErr           error
	providerCancelErr   error
	providerCancelState string
}

func (r *billingCancellationRepository) CancelPaymentOrder(_ context.Context, id, _ string, _ string, _ time.Time) (model.PaymentOrder, bool, error) {
	if r.cancelErr != nil {
		return model.PaymentOrder{}, false, r.cancelErr
	}
	return r.orders[id], r.changed, nil
}

func (r *billingCancellationRepository) SetPaymentProviderCancellation(_ context.Context, _ string, status string, _ time.Time) error {
	r.providerCancelState = status
	return r.providerCancelErr
}

type billingCancellationGateway struct {
	billingGateway
	err   error
	calls int
}

func (g *billingCancellationGateway) Cancel(context.Context, model.PaymentOrder) error {
	g.calls++
	return g.err
}

func TestCancelBestEffortProviderCancellation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	baseOrder := model.PaymentOrder{ID: "order-1", Provider: "bepusdt", ProviderTradeID: ptrString("trade-1")}
	for name, repository := range map[string]Repository{
		"unsupported repository": newBillingRepository(),
		"repository failure":     &billingCancellationRepository{billingRepository: newBillingRepository(), cancelErr: errors.New("cancel failure")},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := newBillingServiceForTest(repository, &billingSettings{}, &billingGateway{}).Cancel(ctx, "order-1", "user-1"); err == nil {
				t.Fatal("Cancel() unexpectedly succeeded")
			}
		})
	}

	for name, gatewayErr := range map[string]error{"success": nil, "provider failure": errors.New("provider failure")} {
		t.Run(name, func(t *testing.T) {
			repository := &billingCancellationRepository{billingRepository: newBillingRepository(), changed: true}
			repository.orders[baseOrder.ID] = baseOrder
			repository.paymentForUser = model.PaymentOrder{ID: baseOrder.ID, Status: "cancelled"}
			gateway := &billingCancellationGateway{err: gatewayErr}
			order, err := newBillingServiceForTest(repository, &billingSettings{}, gateway).Cancel(ctx, baseOrder.ID, "user-1")
			if err != nil || order.ID != baseOrder.ID || gateway.calls != 1 {
				t.Fatalf("Cancel() = (%+v, %v), calls %d", order, err, gateway.calls)
			}
			wantStatus := "cancelled"
			if gatewayErr != nil {
				wantStatus = "failed"
			}
			if repository.providerCancelState != wantStatus {
				t.Fatalf("provider cancellation state = %q, want %q", repository.providerCancelState, wantStatus)
			}
		})
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
