package providerops

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

type fencedDispatcherRepository struct {
	dispatcherRepository
	allowed bool
	err     error
	checks  int
}

func (r *fencedDispatcherRepository) AccountOperationPermitted(context.Context, string) (bool, error) {
	r.checks++
	return r.allowed, r.err
}

func TestDispatcherRetainsFencedNonterminalContinuation(t *testing.T) {
	for _, status := range []Status{StatusQueued, StatusProcessing, StatusPendingReview, StatusPartial} {
		t.Run(string(status), func(t *testing.T) {
			r := &fencedDispatcherRepository{dispatcherRepository: dispatcherRepository{operation: Operation{
				Receipt: model.OperationReceipt{ID: "receipt", Kind: "mutation", Status: string(status)}, Attempts: 1,
			}}}
			d := NewDispatcher(r)
			if err := d.Register("mutation", HandlerFunc(func(context.Context, Operation, model.OutboxJob) error {
				t.Fatal("account fence admitted a new upstream mutation")
				return nil
			})); err != nil {
				t.Fatal(err)
			}
			err := d.HandleOutbox(context.Background(), model.OutboxJob{Payload: `{"operationId":"receipt"}`})
			var wait *outbox.WaitError
			if !errors.As(err, &wait) || wait.Reason != "account_operation_reconciliation_pending" || wait.After != time.Minute {
				t.Fatalf("fenced continuation must remain pending: %v", err)
			}
			if r.operation.Receipt.Status != string(status) || r.operation.Attempts != 1 || r.checks != 1 {
				t.Fatal("dispatcher changed retained attempt evidence")
			}
		})
	}
}

func TestDispatcherTerminalReceiptNeedsNoAccountAdmission(t *testing.T) {
	for _, status := range []Status{StatusSucceeded, StatusFailed, StatusCompensated} {
		r := &fencedDispatcherRepository{dispatcherRepository: dispatcherRepository{operation: Operation{
			Receipt: model.OperationReceipt{ID: "receipt", Kind: "unregistered", Status: string(status)},
		}}}
		if err := NewDispatcher(r).HandleOutbox(context.Background(), model.OutboxJob{Payload: `{"operationId":"receipt"}`}); err != nil || r.checks != 0 {
			t.Fatalf("terminal %s must complete without new admission: %v", status, err)
		}
	}
}

func TestDispatcherPreservesAccountGateError(t *testing.T) {
	want := errors.New("database temporarily unavailable")
	r := &fencedDispatcherRepository{dispatcherRepository: dispatcherRepository{operation: Operation{
		Receipt: model.OperationReceipt{ID: "receipt", Kind: "mutation", Status: string(StatusProcessing)},
	}}, err: want}
	if err := NewDispatcher(r).HandleOutbox(context.Background(), model.OutboxJob{Payload: `{"operationId":"receipt"}`}); !errors.Is(err, want) {
		t.Fatal("account gate failure was swallowed", err)
	}
}
