package httpapi

import (
	"context"
	"fmt"
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

func TestStarsRefundWebhookRequiresExactRetainedChargeBeforeReversal(t *testing.T) {
	for _, proof := range []string{"missing callback charge", "wrong callback charge", "missing retained charge"} {
		t.Run(proof, func(t *testing.T) {
			t.Parallel()
			ctx, now := context.Background(), time.Now().UTC()
			server, _, user := testBrowserHTTP(t)
			store := server.deps.Store
			order := model.PaymentOrder{ID: "proof-order", UserID: user.ID, Provider: "stars", MethodID: "stars", Status: "creating",
				TXBMinor: 1000, PayableAmount: "10", PayableCurrency: "XTR", RateSnapshot: "1", RateDirection: "txb_per_currency", ExpiresAt: now.Add(time.Hour)}
			if _, _, err := store.BeginPaymentOrderOperation(ctx, order, providerops.CreateInput{
				ActorUserID: user.ID, OwnerUserID: user.ID, Kind: billing.OperationCreateKind, IdempotencyKey: "proof-checkout",
				RequestFingerprint: "proof-checkout-fingerprint", Items: []providerops.ItemInput{{Key: "payment", TargetType: "payment_order", TargetID: order.ID}},
			}, now); err != nil {
				t.Fatal(err)
			}
			if _, _, err := store.SettlePayment(ctx, "stars", "charge", "verified", order.ID, "charge", "", now); err != nil {
				t.Fatal(err)
			}
			charge := ""
			if proof == "wrong callback charge" {
				charge = "another-charge"
			} else if proof == "missing retained charge" {
				charge = "charge"
				if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET provider_trade_id=NULL WHERE id=$1`, order.ID); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.PutSetting(ctx, "telegram.webhook_secret", "proof-secret", false, nil); err != nil {
				t.Fatal(err)
			}
			server.deps.Settings = admin.NewSettingsService(store, nil)
			server.deps.Billing = billing.NewService(store, nil, nil, nil, nil)
			body := fmt.Sprintf(`{"update_id":7,"message":{"refunded_payment":{"currency":"XTR","total_amount":10,"invoice_payload":"proof-order","telegram_payment_charge_id":%q}}}`, charge)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/telegram", strings.NewReader(body))
			request.Header.Set(telegram.WebhookSecretHeader, "proof-secret")
			response := httptest.NewRecorder()
			server.telegramWebhook(response, request)
			if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "REFUND_MISMATCH") {
				t.Fatalf("unproven refund accepted: %d %s", response.Code, response.Body.String())
			}
			balance, err := store.Balance(ctx, user.ID)
			if err != nil || balance.Minor != "1000" {
				t.Fatalf("unproven callback reversed wallet: %+v, %v", balance, err)
			}
		})
	}
}
