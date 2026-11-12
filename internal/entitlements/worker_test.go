package entitlements

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"testing"
	"time"
)

func TestWorkerHandleOutboxAppliesEntitlement(t *testing.T) {
	t.Parallel()

	remoteID := "remote-1"
	job := model.OutboxJob{ID: "job-1", Kind: "xnode_apply_entitlement", Payload: `{"purchaseId":"purchase-1"}`, Attempts: 1}
	repository := &entitlementRepository{
		purchase: model.Purchase{ID: "purchase-1", Status: "activating", TrafficLimitBytes: 1234, ResetStrategy: "MONTH", SquadUUIDs: []string{"squad-b", "squad-a"}, ValidUntil: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)},
		user:     model.User{ID: "user-1", ProxyUserID: &remoteID},
	}
	remote := &entitlementProxy{}
	worker := newEntitlementWorkerForTest(repository, remote)

	if err := worker.HandleOutbox(context.Background(), job); err != nil {
		t.Fatalf("HandleOutbox(): %v", err)
	}
	if remote.applyUserID != remoteID || remote.applyLimit != 1234 || remote.applyResetStrategy != "MONTH" || !remote.applyExpiresAt.Equal(time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)) || remote.removeCalls != 1 || remote.resetCalls != 1 {
		t.Fatalf("ApplyEntitlement call = %+v", remote)
	}
	if len(remote.applySquads) != 2 || repository.markedPurchaseID != "purchase-1" || repository.markedSuccess == nil || !*repository.markedSuccess {
		t.Fatalf("apply squads/mark = %v, %q/%v", remote.applySquads, repository.markedPurchaseID, repository.markedSuccess)
	}
}

func TestWorkerHandleOutboxReturnsFailureAndMarksTerminalPurchase(t *testing.T) {
	t.Parallel()

	remoteID := "remote-1"
	job := model.OutboxJob{ID: "job-1", Kind: "xnode_apply_entitlement", Payload: `{"purchaseId":"purchase-1"}`, Attempts: 10}
	repository := &entitlementRepository{
		purchase: model.Purchase{ID: "purchase-1", Status: "active", ValidUntil: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)},
		user:     model.User{ID: "user-1", ProxyUserID: &remoteID},
	}
	remote := &entitlementProxy{applyErr: errors.New("upstream unavailable")}
	worker := newEntitlementWorkerForTest(repository, remote)

	if err := worker.HandleOutbox(context.Background(), job); !errors.Is(err, remote.applyErr) {
		t.Fatalf("HandleOutbox() = %v, want provider failure", err)
	}
	if repository.markedSuccess == nil || *repository.markedSuccess {
		t.Fatalf("terminal purchase success marker = %v, want false", repository.markedSuccess)
	}
}

func TestWorkerRetryDoesNotRedispatchTrafficReset(t *testing.T) {
	t.Parallel()

	remoteID := "remote-1"
	repository := &entitlementRepository{
		purchase:   model.Purchase{ID: "purchase-1", Status: "activating", ValidUntil: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)},
		user:       model.User{ID: "user-1", ProxyUserID: &remoteID},
		resetPhase: "reset",
	}
	remote := &entitlementProxy{}
	if err := newEntitlementWorkerForTest(repository, remote).process(context.Background(), model.OutboxJob{Kind: "xnode_apply_entitlement", Payload: `{"purchaseId":"purchase-1"}`, Attempts: 2}); err != nil {
		t.Fatalf("process(): %v", err)
	}
	if remote.applyCalls != 1 || remote.removeCalls != 0 || remote.resetCalls != 0 {
		t.Fatalf("retry calls apply/remove/reset = %d/%d/%d, want 1/0/0", remote.applyCalls, remote.removeCalls, remote.resetCalls)
	}
}

func TestWorkerResumesDurableTrafficResetPhases(t *testing.T) {
	t.Parallel()

	remoteID := "remote-1"
	future := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		phase      string
		wantRemove int
		wantReset  int
		wantApply  int
	}{
		{phase: "pending", wantRemove: 1, wantReset: 1, wantApply: 1},
		{phase: "quiesced", wantReset: 1, wantApply: 1},
		{phase: "reset", wantApply: 1},
	} {
		t.Run(test.phase, func(t *testing.T) {
			t.Parallel()
			repository := &entitlementRepository{purchase: model.Purchase{ID: "purchase", Status: "activating", ValidUntil: future}, user: model.User{ProxyUserID: &remoteID}, resetPhase: test.phase}
			remote := &entitlementProxy{}
			if err := newEntitlementWorkerForTest(repository, remote).process(context.Background(), model.OutboxJob{Kind: "xnode_apply_entitlement", Payload: `{"purchaseId":"purchase"}`}); err != nil {
				t.Fatalf("process(): %v", err)
			}
			if remote.removeCalls != test.wantRemove || remote.resetCalls != test.wantReset || remote.applyCalls != test.wantApply {
				t.Fatalf("calls remove/reset/apply = %d/%d/%d, want %d/%d/%d", remote.removeCalls, remote.resetCalls, remote.applyCalls, test.wantRemove, test.wantReset, test.wantApply)
			}
		})
	}
}
