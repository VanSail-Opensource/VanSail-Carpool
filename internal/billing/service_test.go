package billing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestServiceCreateOrderByProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider string
		rate     string
		txbMinor int64
		amount   string
		currency string
	}{
		{provider: "ezpay", rate: "0.15375", txbMinor: 123, amount: "8.00", currency: "CNY"},
		{provider: "bepusdt", rate: "8", txbMinor: 101, amount: "0.13", currency: "USD"},
		{provider: "stars", rate: "0.3125", txbMinor: 250, amount: "8", currency: "XTR"},
	}

	for _, test := range tests {
		t.Run(test.provider, func(t *testing.T) {
			t.Parallel()
			repository := newBillingRepository()
			settings := &billingSettings{values: map[string]string{
				"billing." + test.provider + ".enabled":                  "true",
				"billing.rate.txb_per_" + strings.ToLower(test.currency): test.rate,
			}}
			gateway := &billingGateway{}
			service := newBillingServiceForTest(repository, settings, gateway)
			user := model.User{ID: "user-1", TelegramID: 9876}

			order, err := service.CreateOrder(context.Background(), user, " "+strings.ToUpper(test.provider)+" ", test.txbMinor)
			if err != nil {
				t.Fatalf("CreateOrder(): %v", err)
			}
			if order.Provider != test.provider || order.PayableAmount != test.amount || order.PayableCurrency != test.currency {
				t.Fatalf("order = (%q, %q, %q), want (%q, %q, %q)", order.Provider, order.PayableAmount, order.PayableCurrency, test.provider, test.amount, test.currency)
			}
			if repository.updatedOrderID != "order-1" {
				t.Fatalf("updated order ID = %q, want order-1", repository.updatedOrderID)
			}
			if gateway.request.OrderID != "order-1" || gateway.request.TelegramID != user.TelegramID || gateway.request.TXBMinor != test.txbMinor {
				t.Fatalf("gateway request identity = %+v", gateway.request)
			}
			wantNotify := "https://example.test/base/api/v1/webhooks/" + test.provider
			if test.provider == "bepusdt" {
				if !strings.HasPrefix(gateway.request.NotifyURL, wantNotify+"/") {
					t.Fatalf("notify URL = %q", gateway.request.NotifyURL)
				}
			} else if gateway.request.NotifyURL != wantNotify {
				t.Fatalf("notify URL = %q", gateway.request.NotifyURL)
			}
			wantReturn := "https://example.test/base/api/v1/payments/return/" + test.provider + "/order-1"
			if gateway.request.ReturnURL != wantReturn || gateway.request.RedirectURL != wantReturn {
				t.Fatalf("return URLs = %q, %q, want %q", gateway.request.ReturnURL, gateway.request.RedirectURL, wantReturn)
			}
			if order.ExpiresAt != service.now().UTC().Add(30*time.Minute) {
				t.Fatalf("expiry = %s", order.ExpiresAt)
			}
		})
	}
}

func TestServiceCreateOrderPreservesProviderCheckout(t *testing.T) {
	t.Parallel()

	repository := newBillingRepository()
	settings := &billingSettings{values: map[string]string{
		"billing.ezpay.enabled":    "true",
		"billing.rate.txb_per_cny": "1",
	}}
	tradeID, paymentURL, qr := "trade", "https://pay.test/order", "qr-payload"
	expires := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	gateway := &billingGateway{checkout: ProviderCheckout{
		TradeID: &tradeID, PaymentURL: &paymentURL, QRPayload: &qr,
		PayableAmount: "1.0", PayableCurrency: "cny", ExpiresAt: expires,
	}}
	service := newBillingServiceForTest(repository, settings, gateway)

	order, err := service.CreateOrder(context.Background(), model.User{ID: "user-1"}, "ezpay", 100)
	if err != nil {
		t.Fatalf("CreateOrder(): %v", err)
	}
	if order.ProviderTradeID == nil || *order.ProviderTradeID != tradeID || order.PaymentURL == nil || *order.PaymentURL != paymentURL || order.QRPayload == nil || *order.QRPayload != qr {
		t.Fatalf("provider checkout not preserved: %+v", order)
	}
	if order.PayableAmount != "1.00" || order.PayableCurrency != "CNY" || !order.ExpiresAt.Equal(expires) {
		t.Fatalf("provider checkout values not preserved: %+v", order)
	}
}

func TestServiceCreateOrderFailures(t *testing.T) {
	t.Parallel()

	testError := errors.New("test failure")
	tests := []struct {
		name       string
		provider   string
		txbMinor   int64
		configure  func(*billingRepository, *billingSettings, *billingGateway)
		want       error
		wantFailed bool
	}{
		{name: "unsupported provider", provider: "cash", txbMinor: 100, want: ErrInvalidOrder},
		{name: "zero amount", provider: "ezpay", txbMinor: 0, want: ErrInvalidOrder},
		{name: "too large", provider: "ezpay", txbMinor: 100_000_000_01, want: ErrInvalidOrder},
		{name: "disabled", provider: "ezpay", txbMinor: 100, want: ErrProviderDisabled},
		{name: "enabled lookup error", provider: "ezpay", txbMinor: 100, configure: func(_ *billingRepository, settings *billingSettings, _ *billingGateway) {
			settings.errs["billing.ezpay.enabled"] = testError
		}, want: testError},
		{name: "rate lookup error", provider: "ezpay", txbMinor: 100, configure: func(_ *billingRepository, settings *billingSettings, _ *billingGateway) {
			settings.values["billing.ezpay.enabled"] = "true"
			settings.errs["billing.rate.txb_per_cny"] = testError
		}},
		{name: "invalid rate", provider: "ezpay", txbMinor: 100, configure: func(_ *billingRepository, settings *billingSettings, _ *billingGateway) {
			settings.values["billing.ezpay.enabled"] = "true"
			settings.values["billing.rate.txb_per_cny"] = "zero"
		}},
		{name: "zero rate", provider: "ezpay", txbMinor: 100, configure: func(_ *billingRepository, settings *billingSettings, _ *billingGateway) {
			settings.values["billing.ezpay.enabled"] = "true"
			settings.values["billing.rate.txb_per_cny"] = "0"
		}},
		{name: "repository create", provider: "ezpay", txbMinor: 100, configure: func(repository *billingRepository, settings *billingSettings, _ *billingGateway) {
			settings.values["billing.ezpay.enabled"] = "true"
			settings.values["billing.rate.txb_per_cny"] = "1"
			repository.createErr = testError
		}},
		{name: "gateway create", provider: "ezpay", txbMinor: 100, configure: func(_ *billingRepository, settings *billingSettings, gateway *billingGateway) {
			settings.values["billing.ezpay.enabled"] = "true"
			settings.values["billing.rate.txb_per_cny"] = "1"
			gateway.err = testError
		}, wantFailed: true},
		{name: "provider checkout changes immutable price", provider: "ezpay", txbMinor: 100, configure: func(_ *billingRepository, settings *billingSettings, gateway *billingGateway) {
			settings.values["billing.ezpay.enabled"] = "true"
			settings.values["billing.rate.txb_per_cny"] = "1"
			gateway.checkout.PayableAmount = "0.01"
			gateway.checkout.PayableCurrency = "CNY"
		}, want: ErrInvalidOrder, wantFailed: true},
		{name: "provider checkout has incomplete crypto price", provider: "bepusdt", txbMinor: 100, configure: func(_ *billingRepository, settings *billingSettings, gateway *billingGateway) {
			settings.values["billing.bepusdt.enabled"] = "true"
			settings.values["billing.rate.txb_per_usd"] = "1"
			gateway.checkout.ActualCryptoAmount = ptrString("1")
		}, want: ErrInvalidOrder, wantFailed: true},
		{name: "repository update", provider: "ezpay", txbMinor: 100, configure: func(repository *billingRepository, settings *billingSettings, _ *billingGateway) {
			settings.values["billing.ezpay.enabled"] = "true"
			settings.values["billing.rate.txb_per_cny"] = "1"
			repository.updateErr = testError
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := newBillingRepository()
			settings := &billingSettings{values: map[string]string{}, errs: map[string]error{}}
			gateway := &billingGateway{}
			if test.configure != nil {
				test.configure(repository, settings, gateway)
			}
			service := newBillingServiceForTest(repository, settings, gateway)
			_, err := service.CreateOrder(context.Background(), model.User{ID: "user-1"}, test.provider, test.txbMinor)
			if err == nil {
				t.Fatal("CreateOrder() unexpectedly succeeded")
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("CreateOrder() error = %v, want %v", err, test.want)
			}
			if repository.failed != test.wantFailed {
				t.Fatalf("FailPaymentOrder called = %t, want %t", repository.failed, test.wantFailed)
			}
		})
	}
}

func TestServiceDoesNotRetryBEPusdtCreate(t *testing.T) {
	t.Parallel()

	repository := newBillingRepository()
	settings := &billingSettings{values: map[string]string{
		"billing.bepusdt.enabled":  "true",
		"billing.rate.txb_per_usd": "1",
	}}
	gateway := &bepusdtRetryGateway{firstErr: errors.New("ambiguous timeout")}
	service := newBillingServiceForTest(repository, settings, gateway)
	_, err := service.CreateOrder(context.Background(), model.User{ID: "user-1"}, "bepusdt", 100)
	if err == nil {
		t.Fatal("CreateOrder() unexpectedly succeeded")
	}
	if gateway.calls != 1 || len(gateway.orderIDs) != 1 || gateway.orderIDs[0] != "order-1" {
		t.Fatalf("create calls/order IDs = %d/%v, want one call for order-1", gateway.calls, gateway.orderIDs)
	}
}
