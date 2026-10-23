package entitlements

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"testing"
	"time"
)

func TestWorkerDrainAppliesEntitlementAndCompletesJob(t *testing.T) {
	t.Parallel()

	remoteID := "remote-1"
	job := model.OutboxJob{ID: "job-1", Kind: "xnode_apply_entitlement", Payload: `{"purchaseId":"purchase-1"}`, Attempts: 1}
	repository := &entitlementRepository{
		jobs:     []*model.OutboxJob{&job},
		purchase: model.Purchase{ID: "purchase-1", Status: "activating", TrafficLimitBytes: 1234, ResetStrategy: "MONTH", SquadUUIDs: []string{"squad-b", "squad-a"}, ValidUntil: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)},
		user:     model.User{ID: "user-1", ProxyUserID: &remoteID},
	}
	remote := &entitlementProxy{}
	worker := newEntitlementWorkerForTest(repository, remote)

	if err := worker.Drain(context.Background(), 1); err != nil {
		t.Fatalf("Drain(): %v", err)
	}
	if remote.applyUserID != remoteID || remote.applyLimit != 1234 || remote.applyResetStrategy != "MONTH" || !remote.applyExpiresAt.Equal(time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)) || remote.removeCalls != 1 || remote.resetCalls != 1 {
		t.Fatalf("ApplyEntitlement call = %+v", remote)
	}
	if len(remote.applySquads) != 2 || repository.markedPurchaseID != "purchase-1" || repository.markedSuccess == nil || !*repository.markedSuccess {
		t.Fatalf("apply squads/mark = %v, %q/%v", remote.applySquads, repository.markedPurchaseID, repository.markedSuccess)
	}
	if repository.completedJobID != "job-1" || repository.completedAttempts != 1 || repository.completedErr != nil {
		t.Fatalf("completion = %q/%d/%v", repository.completedJobID, repository.completedAttempts, repository.completedErr)
	}
}

func TestWorkerDrainRetriesAndMarksTerminalFailure(t *testing.T) {
	t.Parallel()

	remoteID := "remote-1"
	job := model.OutboxJob{ID: "job-1", Kind: "xnode_apply_entitlement", Payload: `{"purchaseId":"purchase-1"}`, Attempts: 10}
	repository := &entitlementRepository{
		jobs:     []*model.OutboxJob{&job},
		purchase: model.Purchase{ID: "purchase-1", Status: "active", ValidUntil: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)},
		user:     model.User{ID: "user-1", ProxyUserID: &remoteID},
	}
	remote := &entitlementProxy{applyErr: errors.New("upstream unavailable")}
	worker := newEntitlementWorkerForTest(repository, remote)

	if err := worker.Drain(context.Background(), 1); err != nil {
		t.Fatalf("Drain(): %v", err)
	}
	if repository.completedErr == nil {
		t.Fatal("failed job was completed without its processing error")
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

func TestWorkerDrainInfrastructureAndContextErrors(t *testing.T) {
	t.Parallel()

	testError := errors.New("infrastructure failure")
	tests := []struct {
		name      string
		limit     int
		configure func(*entitlementRepository)
		cancel    bool
		wantError bool
	}{
		{name: "empty default batch", limit: 0},
		{name: "claim error", limit: 1, configure: func(repository *entitlementRepository) { repository.claimErr = testError }, wantError: true},
		{name: "complete error", limit: 1, configure: func(repository *entitlementRepository) {
			job := model.OutboxJob{ID: "unknown", Kind: "unknown"}
			repository.jobs = []*model.OutboxJob{&job}
			repository.completeErr = testError
		}, wantError: true},
		{name: "cancelled context", limit: 1, cancel: true, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &entitlementRepository{}
			if test.configure != nil {
				test.configure(repository)
			}
			ctx, cancel := context.WithCancel(context.Background())
			if test.cancel {
				cancel()
			} else {
				defer cancel()
			}
			err := newEntitlementWorkerForTest(repository, &entitlementProxy{}).Drain(ctx, test.limit)
			if test.wantError && err == nil {
				t.Fatal("Drain() unexpectedly succeeded")
			}
			if !test.wantError && err != nil {
				t.Fatalf("Drain(): %v", err)
			}
		})
	}
}
