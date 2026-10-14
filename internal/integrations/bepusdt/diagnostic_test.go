package bepusdt

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLiveCreateCancelDiagnostic is opt-in and creates no local user, ledger,
// or durable credential. It exercises the supplied deployment with a unique,
// unpaid one-dollar transaction and immediately cancels it. Failure output is
// intentionally limited to error types/status codes so secrets and payloads do
// not enter test logs.
func TestLiveCreateCancelDiagnostic(t *testing.T) {
	if os.Getenv("BEPUSDT_DIAGNOSTIC") != "1" {
		t.Skip("set BEPUSDT_DIAGNOSTIC=1 for the explicit live diagnostic")
	}
	baseURL, token := os.Getenv("BEPUSDT_DIAGNOSTIC_URL"), os.Getenv("BEPUSDT_DIAGNOSTIC_TOKEN")
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(token) == "" {
		t.Fatal("live diagnostic URL and token must be supplied through the process environment")
	}
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		t.Fatalf("generate diagnostic order identifier: %T", err)
	}
	client, err := NewClient(baseURL, token)
	if err != nil {
		t.Fatalf("construct diagnostic client: %T", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tradeType := os.Getenv("BEPUSDT_DIAGNOSTIC_TRADE_TYPE")
	if tradeType == "" {
		tradeType = "usdt.trc20"
	}
	transaction, err := client.CreateTransaction(ctx, CreateTransactionRequest{
		OrderID: "txcp-diag-" + hex.EncodeToString(random), Amount: "1", Fiat: "USD", TradeType: tradeType,
		NotifyURL: "https://example.invalid/tx-carpool/bepusdt-diagnostic", RedirectURL: "https://example.invalid/tx-carpool/bepusdt-return",
		Name: "VanSail Carpool non-user diagnostic", TimeoutSeconds: 120,
	})
	if err != nil {
		var apiError *APIError
		if errors.As(err, &apiError) {
			t.Fatalf("live create rejected (http=%d api=%d request_id_present=%t message=%q)", apiError.HTTPStatus, apiError.StatusCode, apiError.RequestID != "", apiError.Message)
		}
		t.Fatalf("live create failed: %T", err)
	}
	if err := client.CancelTransaction(ctx, transaction.TradeID); err != nil {
		var apiError *APIError
		if errors.As(err, &apiError) {
			t.Fatalf("live cancel rejected (http=%d api=%d request_id_present=%t message=%q)", apiError.HTTPStatus, apiError.StatusCode, apiError.RequestID != "", apiError.Message)
		}
		t.Fatalf("live cancel failed: %T", err)
	}
	t.Log("live BEPusdt create/cancel diagnostic succeeded with redacted identifiers")
}
