package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/bepusdt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/ezpay"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func paymentCallbackForTest(t *testing.T, adapter paymentAdapter, provider, key string) (billing.ProviderEvent, error) {
	t.Helper()
	if provider == "ezpay" {
		values := url.Values{"pid": {"merchant"}, "out_trade_no": {"order"}, "trade_no": {"trade"}, "trade_status": {"TRADE_SUCCESS"}, "type": {"alipay"}, "money": {"1.00"}}
		values.Set("sign", ezpay.Sign(values, key))
		event, _, err := adapter.VerifyEZPay(context.Background(), values)
		return event, err
	}
	values := map[string]string{"order_id": "order", "trade_id": "trade", "status": "2", "amount": "1.00", "actual_amount": "1.00", "token": "USDT"}
	values["signature"] = bepusdt.Sign(values, key)
	body, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	event, _, err := adapter.VerifyBEPusdt(context.Background(), body)
	return event, err
}

func TestPaymentCallbackUsesDurableProfileSelection(t *testing.T) {
	t.Parallel()
	for _, provider := range []string{"ezpay", "bepusdt"} {
		for _, legacy := range []bool{false, true} {
			t.Run(provider+"/"+map[bool]string{true: "legacy", false: "scoped"}[legacy], func(t *testing.T) {
				t.Parallel()
				adapter, store := paymentProfileAdapterForTest(t, provider)
				rail := "alipay"
				if provider == "bepusdt" {
					rail = "usdt.trc20"
				}
				method, key, wantID := provider+":second:"+rail, "shared-key", "second"
				if legacy {
					method, key, wantID = provider+":"+rail, "legacy-key", ""
				}
				store.orders["order"] = model.PaymentOrder{ID: "order", Provider: provider, MethodID: method}
				event, err := paymentCallbackForTest(t, adapter, provider, key)
				if err != nil || event.ProfileID != wantID || event.OrderID != "order" {
					t.Fatalf("verified callback = (%+v, %v), want profile %q", event, err, wantID)
				}
				wrongKey := "legacy-key"
				if legacy {
					wrongKey = "shared-key"
				}
				if _, err := paymentCallbackForTest(t, adapter, provider, wrongKey); err == nil {
					t.Fatal("callback accepted another account's signature")
				}
				store.orderErr = errors.New("database unavailable")
				if _, err := paymentCallbackForTest(t, adapter, provider, key); err == nil {
					t.Fatal("callback ignored failed order-account lookup")
				}
			})
		}
	}
}

func TestPaymentCallbackMissingOrderStillAuthenticatesForTombstoneReplay(t *testing.T) {
	t.Parallel()
	for _, provider := range []string{"ezpay", "bepusdt"} {
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			adapter, _ := paymentProfileAdapterForTest(t, provider)
			if _, err := paymentCallbackForTest(t, adapter, provider, "shared-key"); err != nil {
				t.Fatalf("retained profile verification: %v", err)
			}
			if _, err := paymentCallbackForTest(t, adapter, provider, "wrong-key"); err == nil {
				t.Fatal("unknown-order callback bypassed signature verification")
			}
		})
	}
}

func TestExplicitPaymentProfileNeverFallsBackToGlobalCredentials(t *testing.T) {
	t.Parallel()
	for _, provider := range []string{"ezpay", "bepusdt"} {
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			adapter, store := paymentProfileAdapterForTest(t, provider)
			for _, id := range []string{"missing", "first"} {
				store.profiles[0].CredentialCiphertext = "corrupt"
				var err error
				if provider == "ezpay" {
					_, err = adapter.ezpayClient(context.Background(), id, "")
				} else {
					_, err = adapter.bepusdtClient(context.Background(), id, "")
				}
				if err == nil {
					t.Fatalf("profile %q inherited global credentials", id)
				}
			}
		})
	}
}
