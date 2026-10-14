package app

import (
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/bepusdt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
)

func TestNormalizeStarTransactionDirections(t *testing.T) {
	t.Parallel()

	user := telegram.User{ID: 42, FirstName: "Ada"}
	partner := &telegram.TransactionPartnerUser{Type: "user", TransactionType: "invoice_payment", User: user, InvoicePayload: "order-1"}
	tests := []struct {
		name       string
		input      telegram.StarTransaction
		wantOK     bool
		wantRefund bool
	}{
		{name: "incoming payment", input: telegram.StarTransaction{ID: "charge-1", Amount: 25, Source: partner}, wantOK: true},
		{name: "outgoing refund", input: telegram.StarTransaction{ID: "charge-1", Amount: -25, Receiver: partner}, wantOK: true, wantRefund: true},
		{name: "fractional transaction", input: telegram.StarTransaction{ID: "charge-1", Amount: 25, NanostarAmount: 1, Source: partner}},
		{name: "unrelated partner", input: telegram.StarTransaction{ID: "charge-1", Amount: 25, Source: &telegram.TransactionPartnerUser{Type: "fragment"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			event, refund, ok := normalizeStarTransaction(test.input)
			if ok != test.wantOK || refund != test.wantRefund {
				t.Fatalf("normalizeStarTransaction() = (%+v, %t, %t), want ok=%t refund=%t", event, refund, ok, test.wantOK, test.wantRefund)
			}
			if test.wantOK && (event.OrderID != "order-1" || event.TradeID != "charge-1" || event.PayableAmount != "25" || event.TelegramID == nil || *event.TelegramID != 42) {
				t.Fatalf("normalized event = %+v", event)
			}
			if test.wantOK && !test.wantRefund && event.DedupeKey != "charge-1" {
				t.Fatalf("incoming dedupe key = %q", event.DedupeKey)
			}
		})
	}
}

func TestMapBEPusdtCheckoutKeepsOnlySettlementFields(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.August, 10, 14, 0, 0, 0, time.UTC)
	checkout := mapBEPusdtCheckout(&bepusdt.Transaction{
		Fiat: "USD", TradeID: "trade-1", OrderID: "order-1", Amount: "1.00", ActualAmount: "0.95",
		Status: 1, Token: "TReceiveAddress", ExpirationTime: 1200,
	}, "1.00", "usdt.trc20", createdAt)

	if checkout.TradeID == nil || *checkout.TradeID != "trade-1" ||
		checkout.PaymentURL != nil ||
		checkout.ReceivingAddress == nil || *checkout.ReceivingAddress != "TReceiveAddress" {
		t.Fatalf("checkout identity/display fields = %+v", checkout)
	}
	if checkout.ActualCryptoAmount == nil || *checkout.ActualCryptoAmount != "0.95" ||
		checkout.ActualCryptoCurrency == nil || *checkout.ActualCryptoCurrency != "USDT" ||
		checkout.PayableAmount != "1.00" || checkout.PayableCurrency != "USD" {
		t.Fatalf("checkout settlement fields = %+v", checkout)
	}
	if checkout.QRPayload != nil {
		t.Fatalf("checkout QR payload = %q, want nil separate from receiving address", *checkout.QRPayload)
	}
	if want := createdAt.Add(20 * time.Minute); !checkout.ExpiresAt.Equal(want) {
		t.Fatalf("checkout expiry = %s, want %s", checkout.ExpiresAt, want)
	}
}
