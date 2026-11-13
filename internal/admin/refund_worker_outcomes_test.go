package admin

import (
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestRefundWorkerProviderOutcomes(t *testing.T) {
	t.Parallel()
	uncertain := errors.New("provider transport interrupted")
	for _, test := range []struct {
		name       string
		processing bool
		callErr    error
		proofErr   error
		refunded   bool
		definitive bool
		status     providerops.Status
		code       string
		calls      int
		proofs     int
		local      int
	}{
		{name: "confirmed", status: providerops.StatusSucceeded, calls: 1, local: 1},
		{name: "definitive rejection", callErr: uncertain, definitive: true, status: providerops.StatusFailed, code: "REFUND_REJECTED", calls: 1, proofs: 1},
		{name: "ambiguous with proof", callErr: uncertain, refunded: true, status: providerops.StatusSucceeded, calls: 1, proofs: 1, local: 1},
		{name: "ambiguous without proof", callErr: uncertain, status: providerops.StatusPendingReview, code: "REFUND_OUTCOME_AMBIGUOUS", calls: 1, proofs: 1},
		{name: "proof unavailable", callErr: uncertain, proofErr: uncertain, status: providerops.StatusPendingReview, code: "REFUND_OUTCOME_AMBIGUOUS", calls: 1, proofs: 1},
		{name: "previous attempt confirmed", processing: true, refunded: true, status: providerops.StatusSucceeded, proofs: 1, local: 1},
		{name: "previous attempt unresolved", processing: true, status: providerops.StatusPendingReview, code: "REFUND_OUTCOME_AMBIGUOUS", proofs: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newRefundWorkerFixture(t)
			f.refunder.callErr, f.refunder.proofErr = test.callErr, test.proofErr
			f.refunder.refunded, f.refunder.definitive = test.refunded, test.definitive
			if test.processing {
				f.repository.operation.Receipt.Status = string(providerops.StatusProcessing)
				f.repository.item.Status = providerops.StatusProcessing
			}
			if err := f.run(); err != nil {
				t.Fatal(err)
			}
			if f.repository.completion.Status != test.status || f.repository.completion.ErrorCode != test.code ||
				f.refunder.calls != test.calls || f.refunder.proofCalls != test.proofs || f.repository.localRefunds != test.local {
				t.Fatalf("outcome = %+v, provider=%+v, local=%d", f.repository.completion, f.refunder, f.repository.localRefunds)
			}
		})
	}
}

func TestRefundWorkerReconcilesAfterLocalPersistenceFailure(t *testing.T) {
	t.Parallel()
	f := newRefundWorkerFixture(t)
	persistErr := errors.New("local refund persistence failed")
	f.repository.refundErr = persistErr
	if err := f.run(); !errors.Is(err, persistErr) {
		t.Fatalf("first run = %v", err)
	}
	if f.repository.item.Status != providerops.StatusProcessing || f.refunder.calls != 1 {
		t.Fatalf("missing ambiguous attempt: item=%+v, provider=%+v", f.repository.item, f.refunder)
	}
	f.repository.refundErr = nil
	f.refunder.refunded = true
	if err := f.run(); err != nil {
		t.Fatal(err)
	}
	if f.repository.completion.Status != providerops.StatusSucceeded || f.refunder.calls != 1 ||
		f.refunder.proofCalls != 1 || f.repository.localRefunds != 2 {
		t.Fatalf("recovery = %+v, provider=%+v, local=%d", f.repository.completion, f.refunder, f.repository.localRefunds)
	}
}

func TestRefundWorkerAlreadyRefundedAndLocalOnlyOrders(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, provider, status string
		local                  int
	}{
		{name: "already refunded", provider: "stars", status: "refunded"},
		{name: "operator local refund", provider: "ezpay", status: "paid", local: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newRefundWorkerFixture(t)
			f.repository.paymentOrder.Provider, f.repository.paymentOrder.Status = test.provider, test.status
			if err := f.run(); err != nil {
				t.Fatal(err)
			}
			if f.repository.completion.Status != providerops.StatusSucceeded || f.repository.localRefunds != test.local ||
				f.refunder.calls != 0 || f.refunder.proofCalls != 0 {
				t.Fatalf("outcome = %+v, provider=%+v", f.repository.completion, f.refunder)
			}
		})
	}
}

func TestRefundWorkerRequiresStarsReconciliation(t *testing.T) {
	t.Parallel()
	f := newRefundWorkerFixture(t)
	provider := &adminRefunder{}
	f.worker.service.refunder = provider
	if err := f.run(); err != nil {
		t.Fatal(err)
	}
	if f.repository.completion.Status != providerops.StatusPendingReview ||
		f.repository.completion.ErrorCode != "REFUND_RECONCILIATION_UNAVAILABLE" || provider.calls != 0 || f.repository.localRefunds != 0 {
		t.Fatalf("unavailable reconciliation = %+v, provider=%+v", f.repository.completion, provider)
	}
}
