package purchaseops

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestComboControlRecoversEveryTerminalItemBeforeLiveTarget(t *testing.T) {
	for _, status := range []providerops.Status{providerops.StatusSucceeded, providerops.StatusFailed,
		providerops.StatusCompensated, providerops.StatusPendingReview, providerops.StatusPartial} {
		t.Run(string(status), func(t *testing.T) {
			repository := newComboRecoveryRepository()
			repository.item.Status = status
			repository.item.ErrorCode = "RECORDED_OUTCOME"
			repository.item.ProviderReference = "native-operation"
			repository.item.ResultJSON = `{"generation":"9007199254740993"}`
			repository.targetErr = errors.New("target has since been retired")
			sync := &comboRecoverySynchronizer{}
			worker := NewComboControlWorker(repository, sync, nil)
			if err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{}); err != nil {
				t.Fatal(err)
			}
			want := providerops.Completion{Status: status, ErrorCode: repository.item.ErrorCode,
				ProviderReference: repository.item.ProviderReference, ResultJSON: repository.item.ResultJSON}
			if repository.completed != want || repository.targetReads != 0 || sync.calls != 0 {
				t.Fatalf("completion=%+v target reads=%d sync calls=%d", repository.completed, repository.targetReads, sync.calls)
			}
		})
	}
}

func TestComboControlRetryAfterReceiptWriteFailureDoesNotRepeatSync(t *testing.T) {
	repository := newComboRecoveryRepository()
	repository.item.Status = providerops.StatusProcessing
	repository.completeErr = errors.New("receipt write unavailable")
	sync := &comboRecoverySynchronizer{err: errors.New("native synchronization failed")}
	worker := NewComboControlWorker(repository, sync, nil)
	job := model.OutboxJob{Attempts: 10}
	if err := worker.HandleProviderOperation(context.Background(), repository.operation, job); err == nil {
		t.Fatal("expected first receipt write failure")
	}
	if repository.item.Status != providerops.StatusFailed || repository.item.ErrorCode != "ENTITLEMENT_SYNC_FAILED" {
		t.Fatalf("durable item outcome lost: %+v", repository.item)
	}
	repository.completeErr = nil
	repository.targetErr = errors.New("target is no longer available")
	if err := worker.HandleProviderOperation(context.Background(), repository.operation, job); err != nil {
		t.Fatal(err)
	}
	if sync.calls != 1 || repository.targetReads != 1 || repository.completed.Status != providerops.StatusFailed {
		t.Fatalf("sync calls=%d target reads=%d completion=%+v", sync.calls, repository.targetReads, repository.completed)
	}
}

type comboRecoveryRepository struct {
	ComboControlWorkerRepository
	operation   providerops.Operation
	item        providerops.Item
	completed   providerops.Completion
	targetErr   error
	completeErr error
	targetReads int
}

func newComboRecoveryRepository() *comboRecoveryRepository {
	return &comboRecoveryRepository{
		operation: providerops.Operation{Receipt: model.OperationReceipt{ID: "operation", Kind: OperationSquadSwitch, Status: "processing"}, OwnerUserID: "owner"},
		item:      providerops.Item{OperationID: "operation", Key: "purchase", TargetType: "purchase", TargetID: "purchase"},
	}
}

func (r *comboRecoveryRepository) ProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	return []providerops.Item{r.item}, nil
}

func (r *comboRecoveryRepository) PurchaseByID(context.Context, string) (model.Purchase, error) {
	r.targetReads++
	return model.Purchase{ID: "purchase", UserID: "owner", Status: "active"}, r.targetErr
}

func (r *comboRecoveryRepository) CompleteProviderOperationItem(_ context.Context, _, _ string, completion providerops.Completion, _ time.Time) (providerops.Item, error) {
	r.item.Status, r.item.ErrorCode = completion.Status, completion.ErrorCode
	r.item.ProviderReference, r.item.ResultJSON = completion.ProviderReference, completion.ResultJSON
	return r.item, nil
}

func (r *comboRecoveryRepository) CompleteProviderOperation(_ context.Context, _ string, completion providerops.Completion, _ time.Time) (providerops.Operation, error) {
	if r.completeErr != nil {
		return providerops.Operation{}, r.completeErr
	}
	r.completed = completion
	r.operation.Receipt.Status = string(completion.Status)
	return r.operation, nil
}

type comboRecoverySynchronizer struct {
	calls int
	err   error
}

func (s *comboRecoverySynchronizer) HandleOutbox(context.Context, model.OutboxJob) error {
	s.calls++
	return s.err
}
