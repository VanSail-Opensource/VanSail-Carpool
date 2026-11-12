package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestQueueRefundRetainsOwnerIntentAndReplaysBeforeMutableRead(t *testing.T) {
	t.Parallel()
	f := newRefundWorkerFixture(t)
	service := f.worker.service
	first, err := service.QueueRefund(context.Background(), "admin-1", "order-1", " operator request ", "refund-key")
	if err != nil {
		t.Fatal(err)
	}
	input := f.repository.input
	var target refundTarget
	if err := json.Unmarshal([]byte(input.SealedTarget), &target); err != nil {
		t.Fatal(err)
	}
	if input.ActorUserID != "admin-1" || input.OwnerUserID != "user-1" || input.Kind != providerops.KindPaymentRefund ||
		input.IdempotencyKey != "refund-key" || target.Reason != "operator request" || len(input.Items) != 1 ||
		input.Items[0].TargetType != "payment_order" || input.Items[0].TargetID != "order-1" {
		t.Fatalf("queued input = %+v, target=%+v", input, target)
	}
	f.repository.replay = true
	f.repository.paymentOrderErr = errors.New("order read unavailable after accepted command")
	second, err := service.QueueRefund(context.Background(), "admin-1", "order-1", "operator request", "refund-key")
	if err != nil || second.ID != first.ID || f.repository.creates != 1 || f.repository.lookups != 1 ||
		f.refunder.calls != 0 || f.repository.localRefunds != 0 {
		t.Fatalf("queue replay = %+v, %v; repository=%+v", second, err, f.repository)
	}
}

func TestQueueRefundRejectsInvalidReasonAndPropagatesOrderReadFailure(t *testing.T) {
	t.Parallel()
	f := newRefundWorkerFixture(t)
	for _, reason := range []string{" ", strings.Repeat("r", 501)} {
		if _, err := f.worker.service.QueueRefund(context.Background(), "admin-1", "order-1", reason, "refund-key"); err == nil {
			t.Fatalf("invalid reason length %d accepted", len(reason))
		}
	}
	unavailable := errors.New("order unavailable")
	f.repository.paymentOrderErr = unavailable
	if _, err := f.worker.service.QueueRefund(context.Background(), "admin-1", "order-1", "operator request", "refund-key"); !errors.Is(err, unavailable) {
		t.Fatalf("order error = %v", err)
	}
	if f.repository.creates != 0 || f.refunder.calls != 0 || f.repository.localRefunds != 0 {
		t.Fatalf("invalid intent dispatched work: repository=%+v", f.repository)
	}
}

func TestQueueJobRetryUsesDurableActorKey(t *testing.T) {
	t.Parallel()
	f := newRefundWorkerFixture(t)
	f.repository.operation.Receipt.Kind = providerops.KindOutboxRetry
	first, err := f.worker.service.QueueJobRetry(context.Background(), "admin-1", "job-1", "retry-key")
	if err != nil {
		t.Fatal(err)
	}
	f.repository.replay = true
	second, err := f.worker.service.QueueJobRetry(context.Background(), "admin-1", "job-1", "retry-key")
	input := f.repository.input
	if err != nil || first.ID != second.ID || f.repository.creates != 1 || f.repository.retries != 0 ||
		input.ActorUserID != "admin-1" || input.OwnerUserID != "admin-1" || input.Kind != providerops.KindOutboxRetry ||
		input.IdempotencyKey != "retry-key" || len(input.Items) != 1 || input.Items[0].TargetType != "outbox_job" || input.Items[0].TargetID != "job-1" {
		t.Fatalf("retry queue = %+v, %v; input=%+v", second, err, input)
	}
}

func TestJobRetryWorkerPreservesTransientStorageFailures(t *testing.T) {
	t.Parallel()
	unavailable := errors.New("retry storage unavailable")
	for _, test := range []struct {
		name string
		err  error
		code string
	}{
		{name: "missing", err: database.ErrNotFound, code: "OUTBOX_JOB_NOT_FOUND"},
		{name: "conflict", err: database.ErrConflict, code: "OUTBOX_JOB_NOT_RETRYABLE"},
		{name: "transient", err: unavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newRefundWorkerFixture(t)
			f.repository.operation.Receipt.Kind = providerops.KindOutboxRetry
			f.repository.item.TargetType, f.repository.item.TargetID = "outbox_job", "job-1"
			f.repository.retryErr = test.err
			err := f.run()
			if test.code == "" {
				if !errors.Is(err, unavailable) || f.repository.item.Status != providerops.StatusProcessing || f.repository.operationCompletions != 0 {
					t.Fatalf("transient retry = %v, repository=%+v", err, f.repository)
				}
			} else if err != nil || f.repository.completion.Status != providerops.StatusFailed || f.repository.completion.ErrorCode != test.code {
				t.Fatalf("definitive retry = %v, completion=%+v", err, f.repository.completion)
			}
			if f.repository.retries != 1 || f.refunder.calls != 0 {
				t.Fatalf("unexpected dispatches: repository=%+v, provider=%+v", f.repository, f.refunder)
			}
		})
	}
}
