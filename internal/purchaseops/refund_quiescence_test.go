package purchaseops

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestMemberRefundRequiresConfirmedQuiescence(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		itemStatus   providerops.Status
		states       []RemoteState
		quiesceErr   error
		wantStatus   providerops.Status
		wantCode     string
		wantQuiesces int
		wantFinals   int
		wantRestores int
	}{
		{name: "fresh successful admission remains open", itemStatus: providerops.StatusQueued,
			states: []RemoteState{{}}, wantStatus: providerops.StatusPendingReview, wantCode: "REFUND_QUIESCE_AMBIGUOUS", wantQuiesces: 1},
		{name: "processing successful admission remains open", itemStatus: providerops.StatusProcessing,
			states: []RemoteState{{}, {}}, wantStatus: providerops.StatusPendingReview, wantCode: "REFUND_QUIESCE_AMBIGUOUS", wantQuiesces: 1},
		{name: "state reopens after ambiguous call", itemStatus: providerops.StatusQueued,
			states: []RemoteState{{Quiesced: true}, {}}, quiesceErr: errors.New("response lost"),
			wantStatus: providerops.StatusPendingReview, wantCode: "REFUND_QUIESCE_AMBIGUOUS", wantQuiesces: 1},
		{name: "confirmed fresh closure refunds", itemStatus: providerops.StatusQueued,
			states: []RemoteState{{Quiesced: true}}, wantQuiesces: 1, wantFinals: 1},
		{name: "confirmed existing closure recovers", itemStatus: providerops.StatusProcessing,
			states: []RemoteState{{Quiesced: true}}, wantFinals: 1},
		{name: "settled usage restores access", itemStatus: providerops.StatusQueued,
			states:     []RemoteState{{Quiesced: true, UsedTrafficBytes: 1}},
			wantStatus: providerops.StatusFailed, wantCode: ReasonTrafficUsed, wantQuiesces: 1, wantRestores: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newRefundQuiescenceRepository(test.itemStatus, now)
			remote := &refundQuiescenceRemote{states: test.states, quiesceErr: test.quiesceErr}
			worker := NewWorker(repository, remote)
			worker.now = func() time.Time { return now }
			if err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{}); err != nil {
				t.Fatal(err)
			}
			if repository.finalized != test.wantFinals || remote.quiesceCalls != test.wantQuiesces || remote.restoreCalls != test.wantRestores {
				t.Fatalf("finalizations=%d quiesces=%d restores=%d", repository.finalized, remote.quiesceCalls, remote.restoreCalls)
			}
			if repository.completed.Status != test.wantStatus || repository.completed.ErrorCode != test.wantCode {
				t.Fatalf("completion=%+v, want %s/%s", repository.completed, test.wantStatus, test.wantCode)
			}
		})
	}
}

func TestMemberRefundRestoresOnlyAfterDefiniteEligibilityConflict(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	unavailable := errors.New("database unavailable")
	for _, test := range []struct {
		name     string
		cause    error
		restores int
		code     string
	}{
		{name: "eligibility changed", cause: ErrIneligible, restores: 1, code: "REFUND_STATE_CONFLICT"},
		{name: "unknown database outcome", cause: unavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := newRefundQuiescenceRepository(providerops.StatusQueued, now)
			repository.finalizeErr = test.cause
			remote := &refundQuiescenceRemote{states: []RemoteState{{Quiesced: true}}}
			worker := NewWorker(repository, remote)
			worker.now = func() time.Time { return now }
			err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{})
			if test.cause == unavailable && !errors.Is(err, unavailable) {
				t.Fatalf("lost unknown outcome: %v", err)
			}
			if test.cause == ErrIneligible && err != nil {
				t.Fatal(err)
			}
			if repository.finalized != 1 || remote.restoreCalls != test.restores || repository.completed.ErrorCode != test.code {
				t.Fatalf("finalizations=%d restores=%d completion=%+v", repository.finalized, remote.restoreCalls, repository.completed)
			}
		})
	}
}
