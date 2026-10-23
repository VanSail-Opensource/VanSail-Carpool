package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestUserOperationWorkerLeavesAmbiguousOutcomePendingReview(t *testing.T) {
	remoteID := "remote-user"
	repository := newUserOperationRepositoryStub([]string{"user-1"})
	repository.users["user-1"] = model.User{ID: "user-1", ProxyUserID: &remoteID}
	repository.desired["user-1"] = desiredAdminPurchase()
	remote := &entitlementRemoteStub{failures: map[string]error{remoteID: errors.New("connection closed")}}
	worker := NewUserOperationWorker(repository, remote)
	worker.now = func() time.Time { return time.Date(2026, 8, 18, 13, 0, 0, 0, time.UTC) }

	if err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{}); err != nil {
		t.Fatalf("HandleProviderOperation(): %v", err)
	}
	if repository.operation.Receipt.Status != string(providerops.StatusPendingReview) ||
		repository.items[0].Status != providerops.StatusPendingReview {
		t.Fatalf("operation = %+v, items = %+v", repository.operation, repository.items)
	}
	if len(remote.applyCalls) != 1 {
		t.Fatalf("provider calls = %v, want one", remote.applyCalls)
	}
}

func TestUserOperationWorkerSummarizesMixedTargetsAsPartial(t *testing.T) {
	remoteID := "remote-failure"
	repository := newUserOperationRepositoryStub([]string{"local-only", "remote-user"})
	repository.users["local-only"] = model.User{ID: "local-only"}
	repository.users["remote-user"] = model.User{ID: "remote-user", ProxyUserID: &remoteID}
	repository.desired["local-only"] = desiredAdminPurchase()
	repository.desired["remote-user"] = desiredAdminPurchase()
	remote := &entitlementRemoteStub{failures: map[string]error{remoteID: errors.New("timeout")}}
	worker := NewUserOperationWorker(repository, remote)

	if err := worker.HandleProviderOperation(context.Background(), repository.operation, model.OutboxJob{}); err != nil {
		t.Fatalf("HandleProviderOperation(): %v", err)
	}
	if repository.operation.Receipt.Status != string(providerops.StatusPartial) {
		t.Fatalf("operation status = %q, want partial", repository.operation.Receipt.Status)
	}
	statuses := map[providerops.Status]int{}
	for _, item := range repository.items {
		statuses[item.Status]++
	}
	if statuses[providerops.StatusSucceeded] != 1 || statuses[providerops.StatusPendingReview] != 1 {
		t.Fatalf("item statuses = %v", statuses)
	}
}

func newUserOperationRepositoryStub(userIDs []string) *userOperationRepositoryStub {
	items := make([]providerops.Item, 0, len(userIDs))
	for _, id := range userIDs {
		items = append(items, providerops.Item{OperationID: "operation-1", Key: id, TargetType: "user",
			TargetID: id, Status: providerops.StatusQueued})
	}
	return &userOperationRepositoryStub{operation: providerops.Operation{Receipt: model.OperationReceipt{
		ID: "operation-1", Kind: providerops.KindAdminBulkExtension, Status: string(providerops.StatusQueued)}},
		items: items, users: map[string]model.User{}, desired: map[string]*model.Purchase{}}
}

func desiredAdminPurchase() *model.Purchase {
	return &model.Purchase{TrafficLimitBytes: 1_024, ResetStrategy: "MONTH_ROLLING",
		SquadUUIDs: []string{"squad-1"}, ValidUntil: time.Now().Add(24 * time.Hour)}
}
