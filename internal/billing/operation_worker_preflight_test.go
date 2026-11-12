package billing

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestPaymentPreflightReadFailuresRetainUnattemptedIntent(t *testing.T) {
	for _, boundary := range []string{"order", "owner", "credential"} {
		t.Run(boundary, func(t *testing.T) {
			ctx := context.Background()
			store, user := newPaymentOperationStore(t, 9885)
			repository := &failingPaymentRepository{Store: store}
			settings := &billingSettings{values: map[string]string{
				"billing.bepusdt.enabled": "true", "billing.rate.txb_per_usd": "1",
			}, errs: map[string]error{}}
			gateway := &bepusdtRetryGateway{}
			service := newBillingServiceForTest(repository, settings, gateway)
			queued, err := service.QueueOrder(ctx, user, "bepusdt", 100, "preflight-recovery")
			if err != nil {
				t.Fatal(err)
			}
			unavailable := errors.New("local read unavailable")
			switch boundary {
			case "order":
				repository.orderReadErr = unavailable
			case "owner":
				repository.ownerReadErr = unavailable
			case "credential":
				settings.errs["billing.bepusdt.api_token"] = unavailable
			}
			worker, err := NewOperationWorker(service)
			if err != nil {
				t.Fatal(err)
			}
			if err := worker.HandleProviderOperation(ctx, paymentWorkerOperation(t, store, queued), model.OutboxJob{}); !errors.Is(err, unavailable) {
				t.Fatalf("preflight error = %v", err)
			}
			item := paymentWorkerItem(t, store, queued)
			order, err := store.PaymentOrderByID(ctx, queued.PaymentOrderID)
			if err != nil || order.Status != "creating" || item.Status != providerops.StatusQueued || item.AttemptStartedAt != nil || repository.claims != 0 || gateway.calls != 0 {
				t.Fatalf("preflight consumed intent: order=%+v item=%+v claims=%d calls=%d err=%v", order, item, repository.claims, gateway.calls, err)
			}
			repository.orderReadErr, repository.ownerReadErr = nil, nil
			delete(settings.errs, "billing.bepusdt.api_token")
			_, operation := exercisePaymentOperation(t, service, queued)
			if operation.Receipt.Status != string(providerops.StatusSucceeded) || repository.claims != 1 || gateway.calls != 1 {
				t.Fatalf("recovery=%+v claims=%d calls=%d", operation.Receipt, repository.claims, gateway.calls)
			}
		})
	}
}

func TestPaymentExhaustedPreflightRemainsReviewableWithoutProviderCall(t *testing.T) {
	ctx := context.Background()
	store, user := newPaymentOperationStore(t, 9886)
	repository := &failingPaymentRepository{Store: store}
	gateway := &bepusdtRetryGateway{}
	service := newBillingServiceForTest(repository, &billingSettings{values: map[string]string{
		"billing.ezpay.enabled": "true", "billing.rate.txb_per_cny": "1",
	}}, gateway)
	queued, err := service.QueueOrder(ctx, user, "ezpay", 100, "exhausted-preflight")
	if err != nil {
		t.Fatal(err)
	}
	repository.orderReadErr = errors.New("order read unavailable")
	worker, err := NewOperationWorker(service)
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleProviderOperation(ctx, paymentWorkerOperation(t, store, queued), model.OutboxJob{Attempts: 10}); err != nil {
		t.Fatal(err)
	}
	operation := paymentWorkerOperation(t, store, queued)
	if operation.Receipt.Status != string(providerops.StatusPendingReview) || operation.Receipt.ErrorCode == nil || *operation.Receipt.ErrorCode != "PAYMENT_LOCAL_STATE_UNAVAILABLE" || gateway.calls != 0 {
		t.Fatalf("exhausted preflight outcome=%+v calls=%d", operation.Receipt, gateway.calls)
	}
	repository.orderReadErr = nil
	if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{Attempts: 11}); err != nil || gateway.calls != 0 {
		t.Fatalf("review-required command retried provider: calls=%d err=%v", gateway.calls, err)
	}
}
