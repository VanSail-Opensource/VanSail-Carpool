package bepusdt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateTransactionRejectsUnboundedResponseAmount(t *testing.T) {
	t.Parallel()
	for _, amount := range []string{"1e1000000000", "1e-1000000000", strings.Repeat("1", 65), "1/1"} {
		t.Run(amount, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status_code": 200,
					"data": map[string]any{"fiat": "USD", "trade_id": "trade", "order_id": "order", "amount": amount,
						"actual_amount": "1", "status": 1, "token": "address", "expiration_time": 1200},
				})
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "test-token")
			if err != nil {
				t.Fatal(err)
			}
			transaction, err := client.CreateTransaction(context.Background(), CreateTransactionRequest{
				OrderID: "order", Amount: "1", NotifyURL: "https://app.example/notify", RedirectURL: "https://app.example/return",
			})
			if err == nil || transaction != nil {
				t.Fatal("unbounded or non-decimal response amount was accepted")
			}
		})
	}
	if !decimalEquivalent("1.000", "1") || !decimalEquivalent("28.88", "28.880") || !decimalEquivalent("1e+06", "1000000") {
		t.Fatal("bounded decimal normalization was lost")
	}
}
