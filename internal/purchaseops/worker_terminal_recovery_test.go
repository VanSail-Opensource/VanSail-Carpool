package purchaseops

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestMemberOperationRecoversExactTerminalItem(t *testing.T) {
	for _, kind := range []string{OperationResetKind, OperationRefundKind} {
		for _, status := range []providerops.Status{providerops.StatusSucceeded, providerops.StatusFailed,
			providerops.StatusCompensated, providerops.StatusPendingReview, providerops.StatusPartial} {
			t.Run(kind+"/"+string(status), func(t *testing.T) {
				repository := &terminalRecoveryRepository{item: providerops.Item{
					OperationID: "operation", Key: "purchase", TargetType: "purchase", TargetID: "retired-purchase",
					Status: status, ErrorCode: "RECORDED_OUTCOME", ProviderReference: "native-receipt",
					ResultJSON: `{"settled":9007199254740993,"refund":125}`,
				}}
				operation := providerops.Operation{OwnerUserID: "owner", Receipt: model.OperationReceipt{
					ID: "operation", Kind: kind, Status: string(providerops.StatusProcessing),
				}}
				worker := NewWorker(repository, nil)
				want := providerops.Completion{Status: status, ErrorCode: repository.item.ErrorCode,
					ProviderReference: repository.item.ProviderReference, ResultJSON: repository.item.ResultJSON}
				unavailable := errors.New("receipt write unavailable")
				repository.completeErr = unavailable
				if err := worker.HandleProviderOperation(context.Background(), operation, model.OutboxJob{}); !errors.Is(err, unavailable) {
					t.Fatalf("receipt write failure was lost: %v", err)
				}
				repository.completeErr = nil
				if err := worker.HandleProviderOperation(context.Background(), operation, model.OutboxJob{}); err != nil {
					t.Fatal(err)
				}
				if repository.completed != want || repository.completeCalls != 2 {
					t.Fatalf("completion=%+v calls=%d, want %+v", repository.completed, repository.completeCalls, want)
				}
			})
		}
	}
}

// Only recorded-item reads and receipt completion are available. Any attempt to
// reload a mutable target, recheck billing or invoke the provider panics.
type terminalRecoveryRepository struct {
	WorkerRepository
	item          providerops.Item
	completed     providerops.Completion
	completeErr   error
	completeCalls int
}

func (r *terminalRecoveryRepository) ProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	return []providerops.Item{r.item}, nil
}

func (r *terminalRecoveryRepository) CompleteProviderOperation(_ context.Context, _ string, completion providerops.Completion, _ time.Time) (providerops.Operation, error) {
	r.completeCalls++
	r.completed = completion
	return providerops.Operation{}, r.completeErr
}
