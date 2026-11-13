package bepusdt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSignOfficialFixture(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"order_id": "20220201030210321", "amount": "42",
		"notify_url": "http://example.com/notify", "redirect_url": "http://example.com/redirect",
	}
	const token = "epusdt_password_xasddawqe"
	const want = "1cd4b52df5587cfb1968b0c0c6e156cd"
	if got := Sign(values, token); got != want {
		t.Fatalf("Sign() = %q, want official fixture %q", got, want)
	}
}

func TestNewClientTrimsCopiedConfiguration(t *testing.T) {
	t.Parallel()
	client, err := NewClient("  https://pay.example/  ", "  token  ")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client.baseURL.String() != "https://pay.example/" || client.token != "token" {
		t.Fatalf("client configuration was not normalized: %#v", client)
	}
}

func TestProviderNumberStringMatchesUpstreamFloatFormatting(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		raw, want string
	}{
		{raw: "1.00", want: "1"},
		{raw: "28.80", want: "28.8"},
		{raw: "1e3", want: "1000"},
	} {
		if got := providerNumberString(test.raw); got != test.want {
			t.Errorf("providerNumberString(%q) = %q, want %q", test.raw, got, test.want)
		}
	}
}

func TestCreateTransaction(t *testing.T) {
	t.Parallel()
	const token = "epusdt_password_xasddawqe"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/v1/order/create-transaction" {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		defer func() { _ = request.Body.Close() }()
		var raw map[string]json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&raw); err != nil {
			t.Errorf("decode request: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if string(raw["amount"]) != "28.88" {
			t.Errorf("amount JSON = %s, want exact number", raw["amount"])
		}
		values := make(map[string]string, len(raw))
		for key, value := range raw {
			scalar, err := scalarString(value)
			if err != nil {
				t.Errorf("scalarString(%s): %v", key, err)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			values[key] = scalar
		}
		if values["signature"] != Sign(values, token) {
			t.Errorf("signature = %q", values["signature"])
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status_code":"200","message":"success","data":{"fiat":"USD","trade_id":"trade-1","order_id":"order-1","amount":28.88,"actual_amount":"4.25","status":"1","token":"TAddress","expiration_time":"1200"}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, token, WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	transaction, err := client.CreateTransaction(context.Background(), CreateTransactionRequest{
		OrderID: "order-1", Amount: "28.88", Name: "TXB", TimeoutSeconds: 1200,
		NotifyURL: "https://app.example/api/v1/webhooks/bepusdt", RedirectURL: "https://app.example/payments/return",
	})
	if err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}
	if transaction.Fiat != "USD" || transaction.ActualAmount != "4.25" || transaction.Status != 1 || transaction.ExpirationTime != 1200 {
		t.Fatalf("transaction = %#v", transaction)
	}
}

func TestCancelTransactionUsesSignedDocumentedEndpoint(t *testing.T) {
	t.Parallel()
	const token = "cancel-secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/v1/order/cancel-transaction" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode cancellation: %v", err)
		}
		if body["trade_id"] != "trade-1" || body["signature"] != Sign(map[string]string{"trade_id": "trade-1"}, token) {
			t.Fatalf("cancellation body = %#v", body)
		}
		_, _ = writer.Write([]byte(`{"status_code":200,"message":"success","data":{"trade_id":"trade-1"}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, token, WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClient(): %v", err)
	}
	if err := client.CancelTransaction(context.Background(), "trade-1"); err != nil {
		t.Fatalf("CancelTransaction(): %v", err)
	}
}

func TestParseUnsignedWebhookRejectsSignedShape(t *testing.T) {
	t.Parallel()
	valid := []byte(`{"order_id":"order-1","amount":"1.00","actual_amount":"0.25","token":"USDT","status":2,"block_transaction_id":"block-1"}`)
	webhook, err := ParseUnsignedWebhook(valid)
	if err != nil || webhook.OrderID != "order-1" || !webhook.Paid() {
		t.Fatalf("ParseUnsignedWebhook() = %#v, %v", webhook, err)
	}
	if _, err := ParseUnsignedWebhook([]byte(`{"order_id":"order-1","amount":"1","actual_amount":"1","token":"USDT","status":2,"signature":"bad"}`)); err == nil {
		t.Fatal("ParseUnsignedWebhook() accepted signed callback")
	}
}

func TestParseAndVerifyWebhook(t *testing.T) {
	t.Parallel()
	const token = "callback-token"
	values := map[string]string{
		"trade_id": "trade-1", "order_id": "order-1", "amount": "28.88", "actual_amount": "4.25",
		"token": "TAddress", "block_transaction_id": "block-1", "status": "2", "created_at": "2026-08-07T12:00:00Z",
	}
	values["signature"] = Sign(values, token)
	valid, err := json.Marshal(map[string]any{
		"trade_id": values["trade_id"], "order_id": values["order_id"], "amount": json.Number(values["amount"]),
		"actual_amount": values["actual_amount"], "token": values["token"], "block_transaction_id": values["block_transaction_id"],
		"status": 2, "created_at": values["created_at"], "signature": values["signature"],
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	tampered := strings.Replace(string(valid), `"amount":28.88`, `"amount":29.88`, 1)
	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{name: "valid mixed scalars", body: string(valid)},
		{name: "tampered", body: tampered, wantErr: ErrInvalidSignature},
		{name: "duplicate", body: `{"trade_id":"one","trade_id":"two"}`, wantErr: errors.New("duplicate")},
		{name: "nested scalar", body: `{"status":{"value":2}}`, wantErr: errors.New("scalar")},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client, err := NewClient("https://pay.example", token)
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			webhook, err := client.ParseAndVerifyWebhook([]byte(test.body))
			if test.wantErr != nil {
				if err == nil {
					t.Fatal("ParseAndVerifyWebhook() error = nil")
				}
				if errors.Is(test.wantErr, ErrInvalidSignature) && !errors.Is(err, ErrInvalidSignature) {
					t.Fatalf("ParseAndVerifyWebhook() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAndVerifyWebhook() error = %v", err)
			}
			if !webhook.Paid() || webhook.Amount != "28.88" || webhook.ActualAmount != "4.25" {
				t.Fatalf("webhook = %#v", webhook)
			}
		})
	}
}

func TestCreateTransactionAPIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"status_code":400,"message":"invalid signature: secret","request_id":"request-1"}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "secret", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.CreateTransaction(context.Background(), CreateTransactionRequest{
		OrderID: "order", Amount: "1", NotifyURL: "https://app.example/notify", RedirectURL: "https://app.example/return",
	})
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.StatusCode != 400 || apiError.RequestID != "request-1" {
		t.Fatalf("CreateTransaction() error = %#v", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaks API token: %v", err)
	}
}

func TestCreateTransactionRejectsMismatchedSuccessData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
	}{
		{name: "order", data: `{"fiat":"USD","trade_id":"trade-1","order_id":"other","amount":"28.88","actual_amount":"4.25","status":1,"token":"TAddress","expiration_time":1200,"payment_url":"https://pay.example/order"}`},
		{name: "fiat", data: `{"fiat":"CNY","trade_id":"trade-1","order_id":"order-1","amount":"28.88","actual_amount":"4.25","status":1,"token":"TAddress","expiration_time":1200,"payment_url":"https://pay.example/order"}`},
		{name: "fiat amount", data: `{"fiat":"USD","trade_id":"trade-1","order_id":"order-1","amount":"28.89","actual_amount":"4.25","status":1,"token":"TAddress","expiration_time":1200,"payment_url":"https://pay.example/order"}`},
		{name: "crypto amount", data: `{"fiat":"USD","trade_id":"trade-1","order_id":"order-1","amount":"28.88","actual_amount":"0","status":1,"token":"TAddress","expiration_time":1200,"payment_url":"https://pay.example/order"}`},
		{name: "expiration", data: `{"fiat":"USD","trade_id":"trade-1","order_id":"order-1","amount":"28.88","actual_amount":"4.25","status":1,"token":"TAddress","expiration_time":0,"payment_url":"https://pay.example/order"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"status_code":200,"message":"success","data":` + test.data + `}`))
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "secret")
			if err != nil {
				t.Fatalf("NewClient(): %v", err)
			}
			_, err = client.CreateTransaction(context.Background(), CreateTransactionRequest{
				OrderID: "order-1", Amount: "28.88", Fiat: "USD", NotifyURL: server.URL + "/notify", RedirectURL: server.URL + "/return",
			})
			if err == nil {
				t.Fatal("CreateTransaction() accepted mismatched success data")
			}
		})
	}
}
