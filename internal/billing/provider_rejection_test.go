package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/bepusdt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type reflectingPaymentGateway struct {
	calls   int
	request ProviderCreateRequest
}

func (g *reflectingPaymentGateway) Create(_ context.Context, request ProviderCreateRequest) (ProviderCheckout, error) {
	g.calls++
	g.request = request
	return ProviderCheckout{}, fmt.Errorf("gateway rejection: %w", &bepusdt.APIError{
		HTTPStatus: 400, StatusCode: 400,
		Message: "rejected notify_url=" + request.NotifyURL + " signature=signed-request-secret",
	})
}

func TestPaymentRejectionDoesNotPublishOrPersistProviderPayload(t *testing.T) {
	ctx := context.Background()
	store, user := newPaymentOperationStore(t, 98_883)
	gateway := &reflectingPaymentGateway{}
	service := newBillingServiceForTest(store, &billingSettings{values: map[string]string{
		"billing.bepusdt.enabled": "true", "billing.rate.txb_per_usd": "1",
	}}, gateway)
	queued, err := service.QueueOrder(ctx, user, "bepusdt", 100, "provider-rejection")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		order, operation := exercisePaymentOperation(t, service, queued)
		if order.Status != "failed" || operation.Receipt.Status != string(providerops.StatusFailed) ||
			operation.Receipt.ErrorCode == nil || *operation.Receipt.ErrorCode != "PAYMENT_CREATE_REJECTED" {
			t.Fatalf("rejection state = %+v %+v", order, operation.Receipt)
		}
		item := paymentWorkerItem(t, store, queued)
		for _, result := range []string{operation.ResultJSON, item.ResultJSON} {
			var fields map[string]string
			if err := json.Unmarshal([]byte(result), &fields); err != nil {
				t.Fatal(err)
			}
			if len(fields) != 1 || fields["paymentOrderId"] != queued.PaymentOrderID {
				t.Fatalf("unexpected rejection evidence: %s", result)
			}
		}
		receipt, err := store.ProviderOperationForOwner(ctx, queued.Operation.ID, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.ErrorMessage == nil || *receipt.ErrorMessage != "Payment provider rejected this checkout request." {
			t.Fatalf("public rejection = %+v", receipt)
		}
		if strings.Contains(string(encoded), gateway.request.NotifyURL) || strings.Contains(string(encoded), "signed-request-secret") {
			t.Fatal("provider request data escaped through the receipt")
		}
	}
	if gateway.calls != 1 || !strings.Contains(gateway.request.NotifyURL, "/webhooks/bepusdt/") {
		t.Fatalf("callback fixture or replay failed: calls=%d", gateway.calls)
	}
}
