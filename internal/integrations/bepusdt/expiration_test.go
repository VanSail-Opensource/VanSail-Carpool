package bepusdt

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCreateTransactionRejectsUnrepresentableExpiry(t *testing.T) {
	for _, test := range []struct {
		name    string
		seconds int64
		valid   bool
	}{
		{name: "remaining second", seconds: 1, valid: true},
		{name: "normal", seconds: 1200, valid: true},
		{name: "maximum duration", seconds: 9_223_372_036, valid: true},
		{name: "zero"},
		{name: "negative", seconds: -1},
		{name: "overflow boundary", seconds: 9_223_372_037},
		{name: "maximum integer", seconds: 1<<63 - 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w, `{"status_code":200,"data":{"fiat":"USD","trade_id":"trade","order_id":"order","amount":"1","actual_amount":"1","status":1,"token":"address","expiration_time":%d}}`, test.seconds)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "test-token")
			if err != nil {
				t.Fatal(err)
			}
			transaction, err := client.CreateTransaction(context.Background(), CreateTransactionRequest{
				OrderID: "order", Amount: "1", TimeoutSeconds: 1200,
				NotifyURL: "https://app.example/notify", RedirectURL: "https://app.example/return",
			})
			if !test.valid {
				if err == nil || transaction != nil {
					t.Fatal("accepted an expiry that cannot be safely converted")
				}
				if got := (Transaction{ExpirationTime: test.seconds}).ExpiresAt(time.Now()); !got.IsZero() {
					t.Fatal("direct expiry conversion overflowed or accepted invalid input")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			created := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
			expiry := transaction.ExpiresAt(created)
			if expiry.Unix()-created.Unix() != test.seconds || !expiry.After(created) {
				t.Fatalf("expiry changed duration: %s -> %s", created, expiry)
			}
		})
	}
}

func TestCreateTransactionRejectsOverflowingRequestedTimeoutBeforeHTTP(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateTransaction(context.Background(), CreateTransactionRequest{
		OrderID: "order", Amount: "1", TimeoutSeconds: 9_223_372_037,
		NotifyURL: "https://app.example/notify", RedirectURL: "https://app.example/return",
	})
	if err == nil || calls.Load() != 0 {
		t.Fatalf("invalid timeout reached provider: calls=%d error=%v", calls.Load(), err)
	}
}
