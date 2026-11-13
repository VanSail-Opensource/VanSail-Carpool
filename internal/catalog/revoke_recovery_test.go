package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestSubscriptionRevokeRecoversRecordedTerminalItem(t *testing.T) {
	for _, status := range []providerops.Status{providerops.StatusSucceeded, providerops.StatusFailed,
		providerops.StatusCompensated, providerops.StatusPendingReview, providerops.StatusPartial} {
		t.Run(string(status), func(t *testing.T) {
			repository := &revokeRecoveryRepository{item: providerops.Item{Key: "subscription", TargetType: "user",
				TargetID: "owner", Status: status, ErrorCode: "RECORDED_OUTCOME", ProviderReference: "provider-receipt",
				ResultJSON: `{"sequence":9007199254740993}`}}
			service := NewService(repository, nil, time.Minute)
			service.cacheDashboard(model.User{ID: "owner"}, RemoteDashboard{SubscriptionURL: "old-credential"}, time.Now(), service.cacheEpoch)
			operation := providerops.Operation{Receipt: model.OperationReceipt{ID: "operation",
				Kind: providerops.KindSubscriptionRevoke, Status: string(providerops.StatusProcessing)}}
			want := providerops.Completion{Status: status, ErrorCode: repository.item.ErrorCode,
				ProviderReference: repository.item.ProviderReference, ResultJSON: repository.item.ResultJSON}
			unavailable := errors.New("receipt unavailable")
			repository.completeErr = unavailable
			if err := service.HandleProviderOperation(context.Background(), operation, model.OutboxJob{Payload: "retired target"}); !errors.Is(err, unavailable) {
				t.Fatalf("completion write error was lost: %v", err)
			}
			repository.completeErr = nil
			if err := service.HandleProviderOperation(context.Background(), operation, model.OutboxJob{Payload: "retired target"}); err != nil {
				t.Fatal(err)
			}
			if repository.completed != want || repository.completeCalls != 2 || repository.userReads != 0 {
				t.Fatalf("recorded outcome was not replayed: completion=%+v calls=%d reads=%d", repository.completed, repository.completeCalls, repository.userReads)
			}
			if _, exists := service.cache["owner"]; status == providerops.StatusSucceeded && exists {
				t.Fatal("successful rotation recovery retained the old credential cache")
			}
		})
	}
}

func TestSubscriptionRevokeLookupFailureRemainsRetryable(t *testing.T) {
	repository := &revokeRecoveryRepository{item: providerops.Item{Key: "subscription", TargetType: "user", TargetID: "owner",
		Status: providerops.StatusQueued}, userErr: context.DeadlineExceeded}
	service := NewService(repository, nil, 0)
	operation := providerops.Operation{OwnerUserID: "owner", ActorUserID: "owner", Receipt: model.OperationReceipt{ID: "operation", Kind: providerops.KindSubscriptionRevoke,
		Status: string(providerops.StatusQueued)}}
	if err := service.HandleProviderOperation(context.Background(), operation, revokeRecoveryJob(t)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("transient owner lookup became a terminal failure: %v", err)
	}
	if repository.completeCalls != 0 || repository.userReads != 1 {
		t.Fatal("lookup failure changed the operation outcome")
	}
}

func TestSubscriptionRevokeEmptyCredentialDoesNotProveRotation(t *testing.T) {
	for _, status := range []providerops.Status{providerops.StatusQueued, providerops.StatusProcessing} {
		t.Run(string(status), func(t *testing.T) {
			remoteID := "remote"
			repository := &revokeRecoveryRepository{item: providerops.Item{Key: "subscription", TargetType: "user", TargetID: "owner",
				Status: status}, user: model.User{ID: "owner", ProxyUserID: &remoteID, AccountStatus: "active"}}
			remote := &memberRevokeRemote{}
			service := NewService(repository, remote, 0)
			operation := providerops.Operation{OwnerUserID: "owner", ActorUserID: "owner", Receipt: model.OperationReceipt{ID: "operation", Kind: providerops.KindSubscriptionRevoke,
				Status: string(status)}}
			if err := service.HandleProviderOperation(context.Background(), operation, revokeRecoveryJob(t)); err == nil {
				t.Fatal("an absent credential was accepted as proof of rotation")
			}
			if repository.completeCalls != 0 || len(remote.dashboardUsers) != 1 || len(remote.rotationUsers) != 0 {
				t.Fatal("missing credential wrote a terminal outcome")
			}
		})
	}
}

func revokeRecoveryJob(t *testing.T) model.OutboxJob {
	t.Helper()
	target, err := json.Marshal(revokeTarget{PreviousHash: subscriptionHash("original-credential")})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"sealedTarget": string(target)})
	if err != nil {
		t.Fatal(err)
	}
	return model.OutboxJob{Payload: string(payload)}
}

// Unimplemented methods deliberately panic if recovery touches mutable target
// state or re-enters the provider-attempt lifecycle.
type revokeRecoveryRepository struct {
	Repository
	revokeOperationRepository
	item          providerops.Item
	user          model.User
	userErr       error
	userReads     int
	completed     providerops.Completion
	completeErr   error
	completeCalls int
}

func (r *revokeRecoveryRepository) ProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	return []providerops.Item{r.item}, nil
}

func (r *revokeRecoveryRepository) UserByID(context.Context, string) (model.User, error) {
	r.userReads++
	return r.user, r.userErr
}

func (r *revokeRecoveryRepository) CompleteProviderOperation(_ context.Context, _ string, completion providerops.Completion, _ time.Time) (providerops.Operation, error) {
	r.completeCalls++
	r.completed = completion
	return providerops.Operation{}, r.completeErr
}
