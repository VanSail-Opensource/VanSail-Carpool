package billing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/bepusdt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestPaymentTerminalItemRecoversExactProviderRejection(t *testing.T) {
	ctx := context.Background()
	store, user := newPaymentOperationStore(t, 9887)
	repository := &failingPaymentRepository{Store: store}
	gateway := &bepusdtRetryGateway{firstErr: &bepusdt.APIError{Message: "merchant account unavailable"}}
	service := newBillingServiceForTest(repository, &billingSettings{values: map[string]string{
		"billing.bepusdt.enabled": "true", "billing.rate.txb_per_usd": "1",
	}}, gateway)
	queued, err := service.QueueOrder(ctx, user, "bepusdt", 100, "recorded-rejection")
	if err != nil {
		t.Fatal(err)
	}
	repository.receiptErr = errors.New("receipt write unavailable")
	worker, err := NewOperationWorker(service)
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleProviderOperation(ctx, paymentWorkerOperation(t, store, queued), model.OutboxJob{}); !errors.Is(err, repository.receiptErr) {
		t.Fatalf("expected interrupted receipt: %v", err)
	}
	item := paymentWorkerItem(t, store, queued)
	var result map[string]string
	if err := json.Unmarshal([]byte(item.ResultJSON), &result); err != nil {
		t.Fatal(err)
	}
	if item.Status != providerops.StatusFailed || item.ErrorCode != "PAYMENT_CREATE_REJECTED" ||
		len(result) != 1 || result["paymentOrderId"] != queued.PaymentOrderID || strings.Contains(item.ResultJSON, "merchant account unavailable") {
		t.Fatalf("safe provider rejection not retained in item: %+v", item)
	}
	repository.receiptErr = nil
	repository.orderReadErr = errors.New("retired target must not be reloaded")
	if err := worker.HandleProviderOperation(ctx, paymentWorkerOperation(t, store, queued), model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	operation := paymentWorkerOperation(t, store, queued)
	if operation.Receipt.Status != string(providerops.StatusFailed) || operation.Receipt.ErrorMessage == nil ||
		*operation.Receipt.ErrorMessage != "Payment provider rejected this checkout request." {
		t.Fatalf("replayed public rejection changed: %+v", operation.Receipt)
	}
	if operation.ResultJSON != item.ResultJSON || operation.ProviderReference != item.ProviderReference || operation.Receipt.ErrorCode == nil || *operation.Receipt.ErrorCode != item.ErrorCode || gateway.calls != 1 {
		t.Fatalf("recorded outcome changed: operation=%+v item=%+v calls=%d", operation, item, gateway.calls)
	}
}

func TestPaymentCancellationReadsBeforeAttemptAndHonorsLateSettlement(t *testing.T) {
	for _, settled := range []bool{false, true} {
		t.Run(map[bool]string{false: "read recovery", true: "late settlement"}[settled], func(t *testing.T) {
			ctx := context.Background()
			store, user := newPaymentOperationStore(t, 9888)
			repository := &failingPaymentRepository{Store: store}
			gateway := &billingCancellationGateway{billingGateway: billingGateway{
				checkout: ProviderCheckout{TradeID: ptrString("cancel-trade")},
			}}
			service := newBillingServiceForTest(repository, &billingSettings{values: map[string]string{
				"billing.bepusdt.enabled": "true", "billing.rate.txb_per_usd": "1",
			}}, gateway)
			created, err := service.QueueOrder(ctx, user, "bepusdt", 100, "create-cancel-recovery")
			if err != nil {
				t.Fatal(err)
			}
			exercisePaymentOperation(t, service, created)
			queued, err := service.QueueCancellation(ctx, created.PaymentOrderID, user.ID, "cancel-recovery")
			if err != nil {
				t.Fatal(err)
			}
			repository.claims = 0
			repository.orderReadErr = errors.New("order temporarily unreadable")
			worker, err := NewOperationWorker(service)
			if err != nil {
				t.Fatal(err)
			}
			if err := worker.HandleProviderOperation(ctx, paymentWorkerOperation(t, store, queued), model.OutboxJob{}); !errors.Is(err, repository.orderReadErr) {
				t.Fatalf("cancellation read error = %v", err)
			}
			if item := paymentWorkerItem(t, store, queued); item.Status != providerops.StatusQueued || repository.claims != 0 || gateway.calls != 0 {
				t.Fatalf("cancellation attempted before read: item=%+v claims=%d calls=%d", item, repository.claims, gateway.calls)
			}
			repository.orderReadErr = nil
			if settled {
				if _, _, err := store.SettlePayment(ctx, "bepusdt", "late-payment-proof", "payload-hash",
					created.PaymentOrderID, "cancel-trade", "", service.now()); err != nil {
					t.Fatal(err)
				}
			}
			_, operation := exercisePaymentOperation(t, service, queued)
			if settled {
				if gateway.calls != 0 || operation.Receipt.Status != string(providerops.StatusFailed) || operation.Receipt.ErrorCode == nil || *operation.Receipt.ErrorCode != "PAYMENT_ALREADY_SETTLED" {
					t.Fatalf("settled order cancellation=%+v calls=%d", operation.Receipt, gateway.calls)
				}
			} else if gateway.calls != 1 || operation.Receipt.Status != string(providerops.StatusSucceeded) {
				t.Fatalf("cancellation recovery=%+v calls=%d", operation.Receipt, gateway.calls)
			}
		})
	}
}
