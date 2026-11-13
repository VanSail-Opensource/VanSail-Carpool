package catalog

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestSubscriptionRevokeUsesAuthenticatedOwnerWithoutLegacyPointer(t *testing.T) {
	foreign := "another-members-native-identity"
	for _, pointer := range []*string{nil, &foreign} {
		user := model.User{ID: "owner", ProxyUserID: pointer, AccountGeneration: 7, AccountStatus: "active"}
		repository := &memberRevokeRepository{revokeRecoveryRepository: &revokeRecoveryRepository{user: user}}
		remote := &memberRevokeRemote{current: RemoteDashboard{SubscriptionURL: "owned-credential"}}
		service := NewService(repository, remote, time.Minute)
		if _, err := service.QueueSubscriptionRevoke(context.Background(), user, "request"); err != nil {
			t.Fatal(err)
		}
		job := memberRevokeJob(t, repository.created.SealedTarget)
		target, err := revokeOperationTarget(job)
		if err != nil || target.Generation != user.AccountGeneration || target.PreviousHash != subscriptionHash("owned-credential") {
			t.Fatal("admission lost the authenticated generation or credential", target, err)
		}
		if err = service.HandleProviderOperation(context.Background(), repository.operation, job); err != nil {
			t.Fatal(err)
		}
		if repository.completed.Status != providerops.StatusSucceeded || len(remote.rotationUsers) != 1 || len(remote.dashboardUsers) != 2 {
			t.Fatal("member rotation did not complete once", repository.completed, remote)
		}
		for _, observed := range append(remote.dashboardUsers, remote.rotationUsers...) {
			if observed.ID != user.ID || observed.AccountGeneration != user.AccountGeneration {
				t.Fatal("legacy pointer selected a different owner or generation", observed)
			}
		}
	}
}

func TestSubscriptionRevokeSupersededGenerationNeverReadsOrRotatesCredentials(t *testing.T) {
	for _, phase := range []providerops.Status{providerops.StatusQueued, providerops.StatusProcessing} {
		for _, historical := range []bool{false, true} {
			user := model.User{ID: "owner", AccountStatus: "active", AccountGeneration: 8}
			repository := &memberRevokeRepository{revokeRecoveryRepository: &revokeRecoveryRepository{user: user,
				item: providerops.Item{Key: "subscription", TargetType: "user", TargetID: user.ID, Status: phase}}}
			repository.operation = providerops.Operation{OwnerUserID: user.ID, ActorUserID: user.ID,
				Receipt: model.OperationReceipt{ID: "operation", Kind: providerops.KindSubscriptionRevoke, Status: string(phase)}}
			fields := map[string]any{"previousHash": subscriptionHash("retired-credential")}
			if !historical {
				fields["generation"] = int64(7)
			}
			target, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			service := NewService(repository, nil, 0)
			if err := service.HandleProviderOperation(context.Background(), repository.operation, memberRevokeJob(t, string(target))); err != nil {
				t.Fatal(err)
			}
			if repository.completed.Status != providerops.StatusFailed || repository.completed.ErrorCode != "ACCOUNT_GENERATION_SUPERSEDED" || repository.item.Status != providerops.StatusFailed {
				t.Fatal("superseded operation did not terminate without a provider", repository.completed)
			}
		}
	}
}

func memberRevokeJob(t *testing.T, target string) model.OutboxJob {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"sealedTarget": target})
	if err != nil {
		t.Fatal(err)
	}
	return model.OutboxJob{Payload: string(payload)}
}

type memberRevokeRemote struct {
	ProxyClient    // Raw legacy identity access is unavailable and would panic.
	current        RemoteDashboard
	dashboardUsers []model.User
	rotationUsers  []model.User
}

func (r *memberRevokeRemote) DashboardForMember(_ context.Context, user model.User) (RemoteDashboard, error) {
	r.dashboardUsers = append(r.dashboardUsers, user)
	return r.current, nil
}

func (r *memberRevokeRemote) RevokeSubscriptionForMember(_ context.Context, user model.User) (string, error) {
	r.rotationUsers = append(r.rotationUsers, user)
	r.current.SubscriptionURL = "replacement-credential"
	return r.current.SubscriptionURL, nil
}

type memberRevokeRepository struct {
	*revokeRecoveryRepository
	created   providerops.CreateInput
	operation providerops.Operation
}

func (r *memberRevokeRepository) ProviderOperationForActorKey(context.Context, string, string, string, string) (model.OperationReceipt, bool, error) {
	return model.OperationReceipt{}, false, nil
}

func (r *memberRevokeRepository) CreateProviderOperation(_ context.Context, input providerops.CreateInput, _ time.Time) (providerops.Operation, bool, error) {
	r.created = input
	r.operation = providerops.Operation{OwnerUserID: input.OwnerUserID, ActorUserID: input.ActorUserID,
		Receipt: model.OperationReceipt{ID: "operation", Kind: input.Kind, Status: string(providerops.StatusQueued)}}
	r.item = providerops.Item{Key: "subscription", TargetType: "user", TargetID: input.OwnerUserID, Status: providerops.StatusQueued}
	return r.operation, true, nil
}

func (r *memberRevokeRepository) BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error) {
	r.operation.Receipt.Status = string(providerops.StatusProcessing)
	return r.operation, nil
}

func (r *memberRevokeRepository) BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error) {
	r.item.Status = providerops.StatusProcessing
	return r.item, nil
}

func (r *memberRevokeRepository) CompleteProviderOperationItem(_ context.Context, _, _ string, completion providerops.Completion, _ time.Time) (providerops.Item, error) {
	r.item.Status, r.item.ErrorCode = completion.Status, completion.ErrorCode
	return r.item, nil
}
