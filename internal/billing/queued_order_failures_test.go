package billing

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestQueuedOrderAdmissionFailuresNeverCallProvider(t *testing.T) {
	for _, test := range []struct {
		name, method, enabled, rate string
		amount                      int64
		settingsError, createError  bool
		want                        error
	}{
		{name: "unsupported provider", method: "cash", amount: 100, want: ErrInvalidOrder},
		{name: "zero amount", method: "ezpay", amount: 0, want: ErrInvalidOrder},
		{name: "too large", method: "ezpay", amount: 100_000_000_01, want: ErrInvalidOrder},
		{name: "disabled", method: "ezpay", amount: 100, want: ErrProviderDisabled},
		{name: "settings unavailable", method: "ezpay", amount: 100, settingsError: true},
		{name: "missing rate", method: "ezpay", enabled: "true", amount: 100, want: ErrProviderDisabled},
		{name: "invalid rate", method: "ezpay", enabled: "true", rate: "zero", amount: 100, want: ErrProviderDisabled},
		{name: "zero rate", method: "ezpay", enabled: "true", rate: "0", amount: 100, want: ErrProviderDisabled},
		{name: "repository failure", method: "ezpay", enabled: "true", rate: "1", amount: 100, createError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, user := newPaymentOperationStore(t, 9880)
			repository := &failingPaymentRepository{Store: store}
			settings := &billingSettings{values: map[string]string{"billing.ezpay.enabled": test.enabled, "billing.rate.txb_per_cny": test.rate}, errs: map[string]error{}}
			if test.settingsError {
				settings.errs["billing.ezpay.enabled"] = errors.New("settings unavailable")
			}
			if test.createError {
				repository.beginErr = errors.New("intent write unavailable")
			}
			gateway := &billingGateway{}
			service := newBillingServiceForTest(repository, settings, gateway)
			if _, err := service.QueueOrder(context.Background(), user, test.method, test.amount, "rejected"); err == nil || test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("QueueOrder error = %v, want %v", err, test.want)
			}
			if gateway.request.OrderID != "" {
				t.Fatal("rejected admission called provider")
			}
			assertPaymentOperationCount(t, store, "payment_orders", 0)
			assertPaymentOperationCount(t, store, "outbox_jobs", 0)
		})
	}
}

func TestQueuedCheckoutUncertaintyRemainsReconcilable(t *testing.T) {
	for _, test := range []struct {
		name, provider, code string
		checkout             ProviderCheckout
		gatewayErr           error
		storeFailure         bool
	}{
		{name: "ambiguous gateway", provider: "ezpay", code: "PAYMENT_CREATE_AMBIGUOUS", gatewayErr: errors.New("response lost")},
		{name: "conflicting immutable price", provider: "ezpay", code: "PAYMENT_CHECKOUT_REJECTED", checkout: ProviderCheckout{PayableAmount: "0.01", PayableCurrency: "CNY"}},
		{name: "incomplete crypto price", provider: "bepusdt", code: "PAYMENT_CHECKOUT_REJECTED", checkout: ProviderCheckout{ActualCryptoAmount: ptrString("1")}},
		{name: "checkout storage failure", provider: "ezpay", code: "PAYMENT_CHECKOUT_REJECTED", storeFailure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, user := newPaymentOperationStore(t, 9881)
			repository := &failingPaymentRepository{Store: store}
			if test.storeFailure {
				repository.checkoutErr = errors.New("checkout write unavailable")
			}
			settings := &billingSettings{values: map[string]string{"billing." + test.provider + ".enabled": "true", "billing.rate.txb_per_cny": "1", "billing.rate.txb_per_usd": "1"}}
			gateway := &billingGateway{checkout: test.checkout, err: test.gatewayErr}
			service := newBillingServiceForTest(repository, settings, gateway)
			queued, err := service.QueueOrder(context.Background(), user, test.provider, 100, "uncertain")
			if err != nil {
				t.Fatal(err)
			}
			order, operation := exercisePaymentOperation(t, service, queued)
			if order.Status != "creating" || operation.Receipt.Status != string(providerops.StatusPendingReview) || operation.Receipt.ErrorCode == nil || *operation.Receipt.ErrorCode != test.code {
				t.Fatalf("uncertainty lost: %+v %+v", order, operation.Receipt)
			}
		})
	}
}

func TestQueuedBEPusdtCreateDoesNotRetryAmbiguousOutcome(t *testing.T) {
	store, user := newPaymentOperationStore(t, 9882)
	gateway := &bepusdtRetryGateway{firstErr: errors.New("ambiguous timeout")}
	service := newBillingServiceForTest(store, &billingSettings{values: map[string]string{
		"billing.bepusdt.enabled": "true", "billing.rate.txb_per_usd": "1",
	}}, gateway)
	queued, err := service.QueueOrder(context.Background(), user, "bepusdt", 100, "bep-uncertain")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		_, operation := exercisePaymentOperation(t, service, queued)
		if operation.Receipt.Status != string(providerops.StatusPendingReview) {
			t.Fatal("ambiguous outcome changed", operation.Receipt)
		}
	}
	if gateway.calls != 1 || len(gateway.orderIDs) != 1 || gateway.orderIDs[0] != queued.PaymentOrderID {
		t.Fatalf("provider create was repeated: calls=%d IDs=%v", gateway.calls, gateway.orderIDs)
	}
}
