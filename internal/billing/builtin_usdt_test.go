package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/usdt"
)

type builtinBillingFixture struct {
	available bool
	rate      string
}

func (f builtinBillingFixture) Methods(context.Context) ([]model.PaymentMethod, error) {
	return []model.PaymentMethod{{ID: "builtin_usdt:usdt.trc20", Provider: usdt.Provider, Rail: "usdt.trc20", Available: f.available}}, nil
}
func (f builtinBillingFixture) Rate(context.Context) (string, error) { return f.rate, nil }
func (builtinBillingFixture) Expiry(context.Context) (time.Duration, error) {
	return 45 * time.Minute, nil
}

func TestBuiltinUSDTPricingAndMethodsCoexistWithProfiles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repository := newBillingRepository()
	settings := &billingProfileSettings{billingSettings: &billingSettings{values: map[string]string{"billing.rate.txb_per_cny": "10"}}, profiles: []model.PaymentProfile{{ID: "ezpay", Provider: "ezpay", Enabled: true, Configured: true, EnabledChannels: []string{"alipay"}}}}
	service := newBillingServiceForTest(repository, settings, &billingGateway{})
	service.SetBuiltinUSDT(builtinBillingFixture{available: true, rate: "3"})
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
	for _, method := range []string{"builtin_usdt:tron", "builtin_usdt:profile:usdt.trc20", "builtin_usdt:usdc.erc20"} {
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
	service.SetBuiltinUSDT(builtinBillingFixture{rate: "3"})
	if _, err := service.prepareOrder(ctx, model.User{}, "builtin_usdt:usdt.trc20", 100); !errors.Is(err, ErrProviderDisabled) {
		t.Fatal("unhealthy network accepted a new order")
	}
	if code, ok := builtinCreateRejection(usdt.Provider, usdt.ErrCapacity); !ok || code != "USDT_QUOTE_CAPACITY" {
		t.Fatal("capacity rejection was ambiguous")
	}
}
