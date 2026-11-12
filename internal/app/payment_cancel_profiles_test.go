package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/bepusdt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func TestPaymentCancellationUsesRetainedAccountAfterRailDisabled(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, method, key string
	}{
		{"scoped", "bepusdt:second:usdt.trc20", "shared-key"},
		{"legacy", "bepusdt:usdt.trc20", "legacy-key"},
		{"legacy without method", "", "legacy-key"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			adapter, store := paymentProfileAdapterForTest(t, "bepusdt")
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var body map[string]string
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/order/cancel-transaction" || json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Error("unexpected cancellation request")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if body["trade_id"] != "trade" || body["signature"] != bepusdt.Sign(map[string]string{"trade_id": "trade"}, test.key) {
					t.Error("cancellation used a different account or trade")
					w.WriteHeader(http.StatusForbidden)
					return
				}
				_, _ = w.Write([]byte(`{"status_code":200,"data":{"trade_id":"trade"}}`))
			}))
			defer server.Close()
			for i := range store.profiles {
				store.profiles[i].Endpoint = server.URL
				store.profiles[i].EnabledChannels = nil
			}
			adapter.queue = paymentCancellationQueueForTest(t)
			trade := "trade"
			order := model.PaymentOrder{ID: "order", Provider: "bepusdt", ProviderTradeID: &trade, MethodID: test.method, ProviderRail: "usdt.trc20"}
			if err := adapter.Cancel(context.Background(), order); err != nil || calls.Load() != 1 {
				t.Fatalf("cancel retained order: calls=%d err=%v", calls.Load(), err)
			}
		})
	}
}

func TestPaymentCancellationRejectsUnusableAccountWithoutFallback(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"malformed", "ezpay:second:alipay", "bepusdt:missing:usdt.trc20", "bepusdt:first:usdt.trc20"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			adapter, store := paymentProfileAdapterForTest(t, "bepusdt")
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = w.Write([]byte(`{"status_code":200,"data":{"trade_id":"trade"}}`))
			}))
			defer server.Close()
			for i := range store.profiles {
				store.profiles[i].Endpoint = server.URL
			}
			adapter.queue = paymentCancellationQueueForTest(t)
			store.profiles[0].CredentialCiphertext = "corrupt"
			trade := "trade"
			order := model.PaymentOrder{ID: "order", Provider: "bepusdt", ProviderTradeID: &trade, MethodID: method, ProviderRail: "usdt.trc20"}
			if err := adapter.Cancel(context.Background(), order); err == nil || calls.Load() != 0 {
				t.Fatalf("cancellation selected fallback credentials: calls=%d err=%v", calls.Load(), err)
			}
		})
	}
}

func paymentCancellationQueueForTest(t *testing.T) *upstreamqueue.Queue {
	t.Helper()
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "payment-cancel-test", Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err = queue.Start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		if err := queue.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return queue
}
