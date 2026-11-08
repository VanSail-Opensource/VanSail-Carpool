package providerops

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func TestDispatcherRetainsActiveReviewTargets(t *testing.T) {
	for _, status := range []Status{StatusPendingReview, StatusPartial} {
		r := &fencedDispatcherRepository{allowed: true, dispatcherRepository: dispatcherRepository{operation: Operation{
			Receipt: model.OperationReceipt{ID: "receipt", Kind: "mutation", Status: string(status)}, Attempts: 1,
		}}}
		err := NewDispatcher(r).HandleOutbox(context.Background(), model.OutboxJob{Payload: `{"operationId":"receipt","sealedTarget":"encrypted-target"}`})
		var review *outbox.ReviewError
		if !errors.As(err, &review) || review.Reason != "provider_operation_reconciliation_required" {
			t.Fatalf("%s target would be acknowledged without evidence: %v", status, err)
		}
	}
}

func TestDispatcherReloadsReceiptBeforeAcknowledgingNilHandler(t *testing.T) {
	for _, status := range []Status{StatusSucceeded, StatusFailed, StatusCompensated, StatusPendingReview, StatusPartial, StatusProcessing, StatusQueued} {
		t.Run(string(status), func(t *testing.T) {
			r := &dispatcherRepository{operation: Operation{Receipt: model.OperationReceipt{ID: "receipt", Kind: "mutation", Status: string(StatusProcessing)}, Attempts: 1}}
			d := NewDispatcher(r)
			calls := 0
			if err := d.Register("mutation", HandlerFunc(func(_ context.Context, _ Operation, job model.OutboxJob) error {
				calls++
				if job.Payload != `{"operationId":"receipt","sealedTarget":"encrypted-target"}` {
					t.Fatal("lost encrypted reconciliation target")
				}
				r.operation.Receipt.Status = string(status)
				return nil
			})); err != nil {
				t.Fatal(err)
			}
			err := d.HandleOutbox(context.Background(), model.OutboxJob{Payload: `{"operationId":"receipt","sealedTarget":"encrypted-target"}`})
			switch status {
			case StatusSucceeded, StatusFailed, StatusCompensated:
				if err != nil {
					t.Fatal("known completion retained a finished job", err)
				}
			case StatusPendingReview, StatusPartial:
				var review *outbox.ReviewError
				if !errors.As(err, &review) {
					t.Fatal("new review outcome lost its continuation", err)
				}
			default:
				var wait *outbox.WaitError
				if !errors.As(err, &wait) || wait.Reason != "provider_operation_completion_pending" {
					t.Fatal("unfinished receipt completed its queue job", err)
				}
			}
			if calls != 1 {
				t.Fatal("dispatcher repeated the upstream attempt")
			}
		})
	}
}
