package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

type builtinBillingFixture struct {
	available bool
}

func (f builtinBillingFixture) Methods(context.Context) ([]model.PaymentMethod, error) {
	return []model.PaymentMethod{{ID: "builtin_usdt:usdt.trc20", Provider: usdt.Provider, Rail: "usdt.trc20", Available: f.available}}, nil
}
func (builtinBillingFixture) Expiry(context.Context) (time.Duration, error) {
	return 45 * time.Minute, nil
}

func TestBuiltinUSDTPricingAndMethodsCoexistWithProfiles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := newBillingRepository()
	settings := &billingProfileSettings{billingSettings: &billingSettings{values: map[string]string{"billing.rate.txb_per_cny": "10", "billing.rate.txb_per_usd": "3"}}, profiles: []model.PaymentProfile{{ID: "ezpay", Provider: "ezpay", Enabled: true, Configured: true, EnabledChannels: []string{"alipay"}}}}
	service := newBillingServiceForTest(repository, settings, &billingGateway{})
	service.SetBuiltinUSDT(builtinBillingFixture{available: true})
	methods, err := service.Methods(ctx)
	if err != nil || len(methods) != 3 || methods[1].Provider != usdt.Provider || methods[2].Provider != "ezpay" {
		t.Fatalf("methods=%+v %v", methods, err)
	}
	order, err := service.prepareOrder(ctx, model.User{ID: "user"}, "builtin_usdt:usdt.trc20", 100)
	if err != nil || order.PayableAmount != "0.334" || order.PayableCurrency != "USDT" || order.RateSnapshot != "3" || order.ExpiresAt.Sub(service.now()) != 45*time.Minute {
		t.Fatalf("quote=%+v %v", order, err)
	}
	for _, spec := range usdt.Registry {
		if !CanonicalMethodID(usdt.Provider + ":" + spec.Rail) {
			t.Fatal("missing canonical rail", spec.Rail)
		}
	}
	for _, method := range []string{"builtin_usdt:tron", "builtin_usdt:profile:usdt.trc20", "builtin_usdt:usdc.trc20"} {
		if CanonicalMethodID(method) {
			t.Fatal("invalid built-in rail", method)
		}
	}
	if _, err := service.ValidateEvent(ctx, ProviderEvent{Provider: usdt.Provider, OrderID: "order"}); !errors.Is(err, ErrInvalidOrder) {
		t.Fatal("generic callback could settle built-in transfer")
	}
	amount, currency, address := "0.334", "USDT", "wallet"
	if err := validateCryptoCheckout(usdt.Provider, ProviderCheckout{ActualCryptoAmount: &amount, ActualCryptoCurrency: &currency, ReceivingAddress: &address}); err != nil {
		t.Fatal(err)
	}
	if err := validateCryptoCheckout(usdt.Provider, ProviderCheckout{ActualCryptoAmount: &amount, ActualCryptoCurrency: &currency}); err == nil {
		t.Fatal("missing recipient accepted")
	}
	service.SetBuiltinUSDT(builtinBillingFixture{})
	if _, err := service.prepareOrder(ctx, model.User{}, "builtin_usdt:usdt.trc20", 100); !errors.Is(err, ErrProviderDisabled) {
		t.Fatal("unhealthy network accepted a new order")
	}
	if code, ok := builtinCreateRejection(usdt.Provider, usdt.ErrCapacity); !ok || code != "USDT_QUOTE_CAPACITY" {
		t.Fatal("capacity rejection was ambiguous")
	}
}

func TestBuiltinStablecoinsShareUSDAndRejectMissingRate(t *testing.T) {
	settings := &billingSettings{values: map[string]string{"billing.rate.txb_per_usd": "3"}}
	service := newBillingServiceForTest(newBillingRepository(), settings, &billingGateway{})
	for _, provider := range []string{"builtin_usdt", "bepusdt"} {
		rate, err := service.loadNewRate(context.Background(), provider)
		if err != nil || rate.Canonical() != "3" {
			t.Fatal(provider, rate, err)
		}
	}
	delete(settings.values, "billing.rate.txb_per_usd")
	if _, err := service.loadNewRate(context.Background(), "builtin_usdt"); !errors.Is(err, errRateNotConfigured) {
		t.Fatal(err)
	}
}
