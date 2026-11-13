package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestStarsRefundRedeliveryRepairsReceiptWithoutRepeatingWalletReversal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	server, _, user := testBrowserHTTP(t)
	store := server.deps.Store
	now := time.Now().UTC()
	order := model.PaymentOrder{ID: "stars-refund-order", UserID: user.ID, Provider: "stars", MethodID: "stars",
		Status: "creating", TXBMinor: 1000, PayableAmount: "10", PayableCurrency: "XTR", RateSnapshot: "1",
		RateDirection: "txb_per_currency", ExpiresAt: now.Add(time.Hour)}
	if _, _, err := store.BeginPaymentOrderOperation(ctx, order, providerops.CreateInput{
		ActorUserID: user.ID, OwnerUserID: user.ID, Kind: billing.OperationCreateKind,
		IdempotencyKey: "stars-checkout", RequestFingerprint: "stars-checkout-fingerprint",
		Items: []providerops.ItemInput{{Key: "payment", TargetType: "payment_order", TargetID: order.ID}},
	}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdatePaymentCheckout(ctx, order.ID, nil, nil, nil, order.PayableAmount, order.PayableCurrency, order.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.SettlePayment(ctx, "stars", "stars-charge", "verified-payment", order.ID, "stars-charge", "", now); err != nil {
		t.Fatal(err)
	}
	operation, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{
		ActorUserID: user.ID, OwnerUserID: user.ID, Kind: providerops.KindPaymentRefund,
		IdempotencyKey: "stars-refund", RequestFingerprint: "stars-refund-fingerprint",
		Items: []providerops.ItemInput{{Key: "payment", TargetType: "payment_order", TargetID: order.ID}},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "payment", now); err != nil {
		t.Fatal(err)
	}
	ambiguous := providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "UPSTREAM_OUTCOME_AMBIGUOUS"}
	if _, err := store.CompleteProviderOperationItem(ctx, operation.Receipt.ID, "payment", ambiguous, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, operation.Receipt.ID, ambiguous, now); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`CREATE FUNCTION reject_refund_resolution() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected receipt storage failure'; END $$`,
		`CREATE TRIGGER reject_refund_resolution BEFORE UPDATE ON provider_operations FOR EACH ROW WHEN (NEW.kind='payment_refund' AND NEW.status='succeeded') EXECUTE FUNCTION reject_refund_resolution()`,
	} {
		if _, err := store.DB().ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.PutSetting(ctx, "telegram.webhook_secret", "refund-webhook-secret", false, nil); err != nil {
		t.Fatal(err)
	}
	server.deps.Settings = admin.NewSettingsService(store, nil)
	server.deps.Billing = billing.NewService(store, nil, nil, nil, nil)
	server.deps.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	body, err := json.Marshal(telegram.Update{UpdateID: 4217, Message: &telegram.Message{RefundedPayment: &telegram.RefundedPayment{
		Currency: "XTR", TotalAmount: 10, InvoicePayload: order.ID, TelegramPaymentChargeID: "stars-charge",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	deliver := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/telegram", bytes.NewReader(body))
		request.Header.Set(telegram.WebhookSecretHeader, "refund-webhook-secret")
		response := httptest.NewRecorder()
		server.telegramWebhook(response, request)
		return response
	}
	response := deliver()
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "REFUND_RECONCILIATION_PENDING") {
		t.Fatalf("receipt outage acknowledged authoritative refund: %d %s", response.Code, response.Body.String())
	}
	refunded, err := store.PaymentOrderByID(ctx, order.ID)
	if err != nil || refunded.Status != "refunded" {
		t.Fatal("wallet refund was not durably retained before receipt retry", refunded, err)
	}
	retained, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
	if err != nil || retained.Receipt.Status != string(providerops.StatusPendingReview) {
		t.Fatal("failed resolution fabricated a terminal receipt", retained, err)
	}
	if _, err := store.DB().ExecContext(ctx, `DROP TRIGGER reject_refund_resolution ON provider_operations`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if response = deliver(); response.Code != http.StatusNoContent {
			t.Fatalf("refund redelivery could not recover: %d %s", response.Code, response.Body.String())
		}
	}
	retained, err = store.ProviderOperationByID(ctx, operation.Receipt.ID)
	if err != nil || retained.Receipt.Status != string(providerops.StatusSucceeded) {
		t.Fatal("authoritative callback did not repair the receipt", retained, err)
	}
	items, err := store.ProviderOperationItems(ctx, operation.Receipt.ID)
	if err != nil || len(items) != 1 || items[0].Status != providerops.StatusSucceeded || items[0].ProviderReference != "stars-charge" {
		t.Fatal("authoritative callback did not repair item evidence", items, err)
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "0" {
		t.Fatal("redelivery repeated a wallet reversal", balance, err)
	}
	var reversals int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=$1 AND reference_id=$2 AND kind='payment_reversal'`, user.ID, order.ID).Scan(&reversals); err != nil || reversals != 1 {
		t.Fatal("refund does not have exactly one immutable reversal", reversals, err)
	}
}
