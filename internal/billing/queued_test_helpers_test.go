package billing

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// exercisePaymentOperation runs the real durable worker against the queued
// operation. Provider I/O remains an injected fake; SQL state is authoritative.
func exercisePaymentOperation(t *testing.T, service *Service, queued PaymentOperation) (model.PaymentOrder, providerops.Operation) {
	t.Helper()
	ctx := context.Background()
	worker, err := NewOperationWorker(service)
	if err != nil {
		t.Fatal(err)
	}
	worker.now = service.now
	operation, err := worker.repository.ProviderOperationByID(ctx, queued.Operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	operation, err = worker.repository.ProviderOperationByID(ctx, queued.Operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	order, err := service.repository.PaymentOrderByID(ctx, queued.PaymentOrderID)
	if err != nil {
		t.Fatal(err)
	}
	return order, operation
}

type failingPaymentRepository struct {
	*database.Store
	beginErr, checkoutErr                  error
	orderReadErr, ownerReadErr, receiptErr error
	claims                                 int
}

func (r *failingPaymentRepository) BeginPaymentOrderOperation(ctx context.Context, order model.PaymentOrder,
	input providerops.CreateInput, now time.Time) (providerops.Operation, bool, error) {
	if r.beginErr != nil {
		return providerops.Operation{}, false, r.beginErr
	}
	return r.Store.BeginPaymentOrderOperation(ctx, order, input, now)
}

func (r *failingPaymentRepository) UpdatePaymentCheckoutDetails(ctx context.Context, id string,
	trade, paymentURL, qr, address, cryptoAmount, cryptoCurrency *string,
	amount, currency string, expires time.Time) (model.PaymentOrder, error) {
	if r.checkoutErr != nil {
		return model.PaymentOrder{}, r.checkoutErr
	}
	return r.Store.UpdatePaymentCheckoutDetails(ctx, id, trade, paymentURL, qr, address, cryptoAmount, cryptoCurrency, amount, currency, expires)
}

func (r *failingPaymentRepository) PaymentOrderByID(ctx context.Context, id string) (model.PaymentOrder, error) {
	if r.orderReadErr != nil {
		return model.PaymentOrder{}, r.orderReadErr
	}
	return r.Store.PaymentOrderByID(ctx, id)
}

func (r *failingPaymentRepository) UserByID(ctx context.Context, id string) (model.User, error) {
	if r.ownerReadErr != nil {
		return model.User{}, r.ownerReadErr
	}
	return r.Store.UserByID(ctx, id)
}

func (r *failingPaymentRepository) BeginProviderOperationItemAttempt(ctx context.Context, id, key string, now time.Time) (providerops.Item, error) {
	r.claims++
	return r.Store.BeginProviderOperationItemAttempt(ctx, id, key, now)
}

func (r *failingPaymentRepository) CompleteProviderOperation(ctx context.Context, id string, completion providerops.Completion, now time.Time) (providerops.Operation, error) {
	if r.receiptErr != nil {
		return providerops.Operation{}, r.receiptErr
	}
	return r.Store.CompleteProviderOperation(ctx, id, completion, now)
}

func paymentWorkerOperation(t *testing.T, store *database.Store, queued PaymentOperation) providerops.Operation {
	t.Helper()
	operation, err := store.ProviderOperationByID(context.Background(), queued.Operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	return operation
}

func paymentWorkerItem(t *testing.T, store *database.Store, queued PaymentOperation) providerops.Item {
	t.Helper()
	items, err := store.ProviderOperationItems(context.Background(), queued.Operation.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("operation items = %+v, %v", items, err)
	}
	return items[0]
}
