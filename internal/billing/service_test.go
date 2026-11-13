package billing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestQueuedOrderByProvider(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		provider, rate   string
		txbMinor         int64
		amount, currency string
	}{
		{"ezpay", "0.15375", 123, "8.00", "CNY"},
		{"bepusdt", "8", 101, "0.13", "USD"},
		{"stars", "0.3125", 250, "8", "XTR"},
	} {
		t.Run(test.provider, func(t *testing.T) {
			t.Parallel()
			store, user := newPaymentOperationStore(t, 9876)
			settings := &billingSettings{values: map[string]string{
				"billing." + test.provider + ".enabled":                  "true",
				"billing.rate.txb_per_" + strings.ToLower(test.currency): test.rate,
			}}
			gateway := &billingGateway{}
			service := newBillingServiceForTest(store, settings, gateway)
			queued, err := service.QueueOrder(context.Background(), user, " "+strings.ToUpper(test.provider)+" ", test.txbMinor, "provider-checkout")
			if err != nil {
				t.Fatal(err)
			}
			if gateway.request.OrderID != "" {
				t.Fatal("provider called before queued worker")
			}
			order, operation := exercisePaymentOperation(t, service, queued)
			if operation.Receipt.Status != string(providerops.StatusSucceeded) || order.Status != "pending" {
				t.Fatalf("checkout failed: %+v %+v", order, operation.Receipt)
			}
			if order.Provider != test.provider || order.PayableAmount != test.amount || order.PayableCurrency != test.currency {
				t.Fatalf("incorrect immutable pricing: %+v", order)
			}
			if gateway.request.OrderID != order.ID || gateway.request.TelegramID != user.TelegramID || gateway.request.TXBMinor != test.txbMinor {
				t.Fatalf("incorrect provider identity: %+v", gateway.request)
			}
			notify := "https://example.test/base/api/v1/webhooks/" + test.provider
			if test.provider == "bepusdt" {
				if !strings.HasPrefix(gateway.request.NotifyURL, notify+"/") {
					t.Fatal("callback capability missing")
				}
			} else if gateway.request.NotifyURL != notify {
				t.Fatal("callback destination changed")
			}
			assertCheckoutReturnURL(t, service, gateway.request, order)
			if !order.ExpiresAt.Equal(service.now().UTC().Add(30 * time.Minute)) {
				t.Fatal("incorrect expiry", order.ExpiresAt)
			}
		})
	}
}

func TestQueuedOrderPreservesProviderCheckout(t *testing.T) {
	t.Parallel()
	store, user := newPaymentOperationStore(t, 9877)
	settings := &billingSettings{values: map[string]string{"billing.ezpay.enabled": "true", "billing.rate.txb_per_cny": "1"}}
	trade, paymentURL, qr := "trade", "https://pay.test/order", "qr-payload"
	expires := billingTestNow().Add(45 * time.Minute)
	gateway := &billingGateway{checkout: ProviderCheckout{TradeID: &trade, PaymentURL: &paymentURL, QRPayload: &qr,
		PayableAmount: "1.0", PayableCurrency: "cny", ExpiresAt: expires}}
	service := newBillingServiceForTest(store, settings, gateway)
	queued, err := service.QueueOrder(context.Background(), user, "ezpay", 100, "preserved-checkout")
	if err != nil {
		t.Fatal(err)
	}
	order, operation := exercisePaymentOperation(t, service, queued)
	if operation.Receipt.Status != string(providerops.StatusSucceeded) || order.ProviderTradeID == nil || *order.ProviderTradeID != trade || order.PaymentURL == nil || *order.PaymentURL != paymentURL || order.QRPayload == nil || *order.QRPayload != qr {
		t.Fatalf("checkout not preserved: %+v %+v", order, operation.Receipt)
	}
	if order.PayableAmount != "1.00" || order.PayableCurrency != "CNY" || !order.ExpiresAt.Equal(expires) {
		t.Fatalf("canonical checkout changed: %+v", order)
	}
}
