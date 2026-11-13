package admin

import (
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestRefundWorkerLocalReadFailureDoesNotClaimProviderAttempt(t *testing.T) {
	t.Parallel()
	f := newRefundWorkerFixture(t)
	unavailable := errors.New("database temporarily unavailable")
	f.repository.paymentOrderErr = unavailable
	if err := f.run(); !errors.Is(err, unavailable) {
		t.Fatalf("first run = %v", err)
	}
	if f.repository.item.Status != providerops.StatusQueued || f.repository.operation.Receipt.Status != string(providerops.StatusQueued) ||
		f.repository.itemAttempts != 0 || f.repository.operationAttempts != 0 || f.refunder.calls != 0 || f.repository.localRefunds != 0 {
		t.Fatalf("local outage claimed a provider attempt: repository=%+v, provider=%+v", f.repository, f.refunder)
	}
	f.repository.paymentOrderErr = nil
	if err := f.run(); err != nil {
		t.Fatal(err)
	}
	if f.repository.completion.Status != providerops.StatusSucceeded || f.refunder.calls != 1 ||
		f.refunder.proofCalls != 0 || f.repository.localRefunds != 1 || f.repository.itemAttempts != 1 ||
		f.repository.refundActor == nil || *f.repository.refundActor != "admin-1" {
		t.Fatalf("recovery = %+v, provider=%+v", f.repository, f.refunder)
	}
}

func TestRefundWorkerMissingOrderIsDefinitive(t *testing.T) {
	t.Parallel()
	f := newRefundWorkerFixture(t)
	f.repository.paymentOrderErr = database.ErrNotFound
	if err := f.run(); err != nil {
		t.Fatal(err)
	}
	if f.repository.completion.Status != providerops.StatusFailed || f.repository.completion.ErrorCode != "PAYMENT_NOT_FOUND" ||
		f.refunder.calls != 0 || f.repository.localRefunds != 0 {
		t.Fatalf("missing order outcome = %+v", f.repository.completion)
	}
}

func TestRefundWorkerRecoversItemAfterReceiptCommitFailure(t *testing.T) {
	t.Parallel()
	for _, initialStatus := range []string{"paid", "expired"} {
		t.Run(initialStatus, func(t *testing.T) {
			t.Parallel()
			f := newRefundWorkerFixture(t)
			f.repository.paymentOrder.Status = initialStatus
			persistErr := errors.New("receipt persistence failed")
			f.repository.operationCompletionErr = persistErr
			if err := f.run(); !errors.Is(err, persistErr) {
				t.Fatalf("first run = %v", err)
			}
			want := providerops.Completion{Status: providerops.StatusSucceeded}
			wantCalls := 1
			if initialStatus != "paid" {
				want.Status, want.ErrorCode = providerops.StatusFailed, "PAYMENT_NOT_REFUNDABLE"
				wantCalls = 0
			}
			if f.repository.item.Status != want.Status {
				t.Fatalf("item was not persisted: %+v", f.repository.item)
			}
			// A later payment or a storage outage must not reclassify the recorded outcome.
			f.repository.paymentOrder.Status = "paid"
			f.repository.paymentOrderErr = errors.New("mutable order must not be loaded")
			f.repository.operationCompletionErr = nil
			if err := f.run(); err != nil {
				t.Fatal(err)
			}
			if f.repository.completion != want || f.repository.lookups != 1 || f.repository.itemCompletions != 1 ||
				f.repository.localRefunds != wantCalls || f.refunder.calls != wantCalls {
				t.Fatalf("replayed mutable work: repository=%+v, provider=%+v", f.repository, f.refunder)
			}
		})
	}
}

func TestRefundWorkerRestoresEveryRecordedTerminalOutcome(t *testing.T) {
	t.Parallel()
	for _, status := range []providerops.Status{
		providerops.StatusSucceeded, providerops.StatusFailed, providerops.StatusCompensated,
		providerops.StatusPendingReview, providerops.StatusPartial,
	} {
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			f := newRefundWorkerFixture(t)
			f.repository.operation.Receipt.Status = string(providerops.StatusProcessing)
			want := providerops.Completion{
				Status: status, ProviderReference: "retained-proof", ErrorCode: "RETAINED_OUTCOME",
				ResultJSON: "{\"amountMinor\":\"9007199254740993\"}",
			}
			f.repository.item.Status, f.repository.item.ErrorCode = status, want.ErrorCode
			f.repository.item.ProviderReference, f.repository.item.ResultJSON = want.ProviderReference, want.ResultJSON
			f.repository.paymentOrderErr = errors.New("order unavailable")
			f.job.Payload = "{}"
			if err := f.run(); err != nil {
				t.Fatal(err)
			}
			if f.repository.completion != want || f.repository.lookups != 0 || f.repository.itemAttempts != 0 ||
				f.repository.itemCompletions != 0 || f.refunder.calls != 0 || f.refunder.proofCalls != 0 || f.repository.localRefunds != 0 {
				t.Fatalf("terminal replay = %+v, provider=%+v", f.repository, f.refunder)
			}
		})
	}
}
