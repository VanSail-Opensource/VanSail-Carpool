package entitlements

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestPrepareContinuityExtendsExpiryWithoutReset(t *testing.T) {
	t.Parallel()
	remoteID := "remote-1"
	extended := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	current := model.Purchase{ID: "current", TrafficLimitBytes: 1024, ResetStrategy: "MONTH_ROLLING",
		SquadUUIDs: []string{"squad-a"}, ValidUntil: extended}
	repository := &entitlementRepository{continuity: &current, user: model.User{RemnaUserID: &remoteID}}
	remote := &entitlementRemnawave{}
	job := model.OutboxJob{Kind: jobpayload.ContinuityKind, Payload: `{"purchaseId":"successor"}`}
	if err := newEntitlementWorkerForTest(repository, remote).process(context.Background(), job); err != nil {
		t.Fatalf("process(continuity): %v", err)
	}
	if remote.applyCalls != 1 || remote.removeCalls != 0 || remote.resetCalls != 0 {
		t.Fatalf("calls apply/remove/reset = %d/%d/%d", remote.applyCalls, remote.removeCalls, remote.resetCalls)
	}
	if !remote.applyExpiresAt.Equal(extended) || remote.applyResetStrategy != "MONTH_ROLLING" {
		t.Fatalf("continuity apply = expiry %s strategy %q", remote.applyExpiresAt, remote.applyResetStrategy)
	}
}

func TestPrepareContinuityIsNoopAfterBoundary(t *testing.T) {
	t.Parallel()
	repository := &entitlementRepository{}
	remote := &entitlementRemnawave{}
	job := model.OutboxJob{Kind: jobpayload.ContinuityKind, Payload: `{"purchaseId":"successor"}`}
	if err := newEntitlementWorkerForTest(repository, remote).process(context.Background(), job); err != nil {
		t.Fatalf("process(continuity): %v", err)
	}
	if remote.applyCalls != 0 || remote.removeCalls != 0 || remote.resetCalls != 0 {
		t.Fatalf("unexpected provider calls: %+v", remote)
	}
}
