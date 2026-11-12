package admin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type refundWorkerRepository struct {
	*adminMutationRepository
	item                   providerops.Item
	lookups                int
	operationAttempts      int
	itemAttempts           int
	localRefunds           int
	itemCompletions        int
	operationCompletions   int
	operationCompletionErr error
	completion             providerops.Completion
	retryErr               error
	retries                int
}

func (r *refundWorkerRepository) ProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	return []providerops.Item{r.item}, nil
}

func (r *refundWorkerRepository) BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error) {
	r.operationAttempts++
	r.operation.Receipt.Status = string(providerops.StatusProcessing)
	return r.operation, nil
}

func (r *refundWorkerRepository) BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error) {
	if r.item.Status != providerops.StatusQueued {
		return providerops.Item{}, database.ErrConflict
	}
	r.itemAttempts++
	r.item.Status = providerops.StatusProcessing
	return r.item, nil
}

func (r *refundWorkerRepository) PaymentOrderByID(context.Context, string) (model.PaymentOrder, error) {
	r.lookups++
	return r.paymentOrder, r.paymentOrderErr
}

func (r *refundWorkerRepository) RefundPayment(_ context.Context, actor *string, _, _ string, _ time.Time) (model.PaymentOrder, error) {
	r.localRefunds++
	if r.refundErr != nil {
		return model.PaymentOrder{}, r.refundErr
	}
	r.refundActor = actor
	r.paymentOrder.Status = "refunded"
	return r.paymentOrder, nil
}

func (r *refundWorkerRepository) CompleteProviderOperationItem(_ context.Context, _, _ string, completion providerops.Completion, _ time.Time) (providerops.Item, error) {
	r.itemCompletions++
	r.item.Status, r.item.ErrorCode = completion.Status, completion.ErrorCode
	r.item.ResultJSON, r.item.ProviderReference = completion.ResultJSON, completion.ProviderReference
	return r.item, nil
}

func (r *refundWorkerRepository) CompleteProviderOperation(_ context.Context, _ string, completion providerops.Completion, _ time.Time) (providerops.Operation, error) {
	r.operationCompletions++
	if r.operationCompletionErr != nil {
		return providerops.Operation{}, r.operationCompletionErr
	}
	r.completion = completion
	r.operation.Receipt.Status = string(completion.Status)
	return r.operation, nil
}

func (r *refundWorkerRepository) CompleteOutboxRetryOperation(context.Context, string, string, string, time.Time) error {
	r.retries++
	return r.retryErr
}

type refundReconciler struct {
	calls      int
	proofCalls int
	callErr    error
	proofErr   error
	refunded   bool
	definitive bool
}

func (r *refundReconciler) RefundProvider(context.Context, model.PaymentOrder) error {
	r.calls++
	return r.callErr
}

func (r *refundReconciler) ProviderRefunded(context.Context, model.PaymentOrder) (bool, error) {
	r.proofCalls++
	return r.refunded, r.proofErr
}

func (r *refundReconciler) DefinitiveRefundFailure(error) bool { return r.definitive }

type refundWorkerFixture struct {
	repository *refundWorkerRepository
	refunder   *refundReconciler
	worker     *MutationWorker
	job        model.OutboxJob
}

func newRefundWorkerFixture(t *testing.T) refundWorkerFixture {
	t.Helper()
	repository := &refundWorkerRepository{
		adminMutationRepository: &adminMutationRepository{
			adminCatalogRepository: &adminCatalogRepository{paymentOrder: model.PaymentOrder{
				ID: "order-1", UserID: "user-1", Provider: "stars", Status: "paid",
			}},
			operation: providerops.Operation{
				ActorUserID: "admin-1", OwnerUserID: "user-1",
				Receipt: model.OperationReceipt{
					ID: "operation-1", Kind: providerops.KindPaymentRefund, Status: string(providerops.StatusQueued),
				},
			},
		},
		item: providerops.Item{
			OperationID: "operation-1", Key: "target", TargetType: "payment_order",
			TargetID: "order-1", Status: providerops.StatusQueued,
		},
	}
	refunder := &refundReconciler{}
	service := newAdminServiceForTest(repository, nil, nil, nil, refunder)
	worker, err := NewMutationWorker(service)
	if err != nil {
		t.Fatal(err)
	}
	worker.now = service.now
	target, err := json.Marshal(refundTarget{Reason: "operator request"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"operationId": "operation-1", "sealedTarget": string(target)})
	if err != nil {
		t.Fatal(err)
	}
	return refundWorkerFixture{repository: repository, refunder: refunder, worker: worker, job: model.OutboxJob{Payload: string(payload)}}
}

func (f refundWorkerFixture) run() error {
	return f.worker.HandleProviderOperation(context.Background(), f.repository.operation, f.job)
}
