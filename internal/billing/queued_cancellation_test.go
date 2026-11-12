package billing

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type billingCancellationGateway struct {
	billingGateway
	cancelErr error
	calls     int
}

func (g *billingCancellationGateway) Cancel(context.Context, model.PaymentOrder) error {
	g.calls++
	return g.cancelErr
}

func TestQueuedCancellationRetainsAmbiguousProviderOutcome(t *testing.T) {
	for _, test := range []struct {
		name, providerStatus string
		operationStatus      providerops.Status
		cancelErr            error
		unsupported          bool
	}{
		{name: "success", providerStatus: "cancelled", operationStatus: providerops.StatusSucceeded},
		{name: "ambiguous", providerStatus: "pending_review", operationStatus: providerops.StatusPendingReview, cancelErr: errors.New("response lost")},
		{name: "unsupported", providerStatus: "unsupported", operationStatus: providerops.StatusSucceeded, unsupported: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			store, user := newPaymentOperationStore(t, 9883)
			gateway := &billingCancellationGateway{billingGateway: billingGateway{checkout: ProviderCheckout{TradeID: ptrString("trade-1")}}, cancelErr: test.cancelErr}
			service := newBillingServiceForTest(store, &billingSettings{values: map[string]string{
				"billing.bepusdt.enabled": "true", "billing.rate.txb_per_usd": "1",
			}}, gateway)
			created, err := service.QueueOrder(ctx, user, "bepusdt", 100, "create-for-cancel")
			if err != nil {
				t.Fatal(err)
			}
			_, creation := exercisePaymentOperation(t, service, created)
			if creation.Receipt.Status != string(providerops.StatusSucceeded) {
				t.Fatal("fixture checkout did not succeed", creation.Receipt)
			}
			if test.unsupported {
				service.gateway = &billingGateway{}
			}
			queued, err := service.QueueCancellation(ctx, created.PaymentOrderID, user.ID, "cancel-key")
			if err != nil {
				t.Fatal(err)
			}
			if gateway.calls != 0 {
				t.Fatal("provider cancellation ran before the queued worker")
			}
			for range 2 {
				order, operation := exercisePaymentOperation(t, service, queued)
				if order.Status != "cancelled" || order.ProviderCancelStatus != test.providerStatus || operation.Receipt.Status != string(test.operationStatus) {
					t.Fatalf("cancellation outcome lost: %+v %+v", order, operation.Receipt)
				}
			}
			wantCalls := 1
			if test.unsupported {
				wantCalls = 0
			}
			if gateway.calls != wantCalls {
				t.Fatalf("provider calls = %d, want %d", gateway.calls, wantCalls)
			}
		})
	}
}

func TestQueuedCancellationRejectsUnavailableRepositoryAndMissingOrder(t *testing.T) {
	service := newBillingServiceForTest(newBillingRepository(), nil, nil)
	if _, err := service.QueueCancellation(context.Background(), "order", "owner", "cancel"); err == nil {
		t.Fatal("cancellation accepted a repository without durable operations")
	}
	store, user := newPaymentOperationStore(t, 9884)
	service = paymentOperationService(store, &billingGateway{})
	if _, err := service.QueueCancellation(context.Background(), "missing", user.ID, "cancel"); err == nil {
		t.Fatal("missing order cancellation succeeded")
	}
	assertPaymentOperationCount(t, store, "outbox_jobs", 0)
}
