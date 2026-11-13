package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestSubscriptionRevokeGenerationChangesCannotProveRotation(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		changeAtRead int
		callErr      error
		reads        int
		rotations    int
	}{
		{"initial credential read", 1, nil, 1, 0},
		{"atomic generation rejection", 0, database.ErrAccountUnavailable, 1, 1},
		{"uncertain mutation reconciliation", 2, errors.New("response interrupted"), 2, 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			user := model.User{ID: "owner", AccountStatus: "active", AccountGeneration: 7}
			repository := &memberRevokeRepository{revokeRecoveryRepository: &revokeRecoveryRepository{user: user,
				item: providerops.Item{Key: "subscription", TargetType: "user", TargetID: user.ID, Status: providerops.StatusQueued}}}
			repository.operation = providerops.Operation{OwnerUserID: user.ID, ActorUserID: user.ID,
				Receipt: model.OperationReceipt{ID: "operation", Kind: providerops.KindSubscriptionRevoke, Status: string(providerops.StatusQueued)}}
			remote := &generationRevokeRemote{repository: repository, changeAtRead: scenario.changeAtRead, callErr: scenario.callErr}
			service := NewService(repository, remote, 0)
			target := `{"previousHash":"` + subscriptionHash("original-credential") + `","generation":7}`
			if err := service.HandleProviderOperation(context.Background(), repository.operation, memberRevokeJob(t, target)); err != nil {
				t.Fatal(err)
			}
			if repository.completed.Status != providerops.StatusFailed || repository.completed.ErrorCode != "ACCOUNT_GENERATION_SUPERSEDED" ||
				remote.reads != scenario.reads || remote.rotations != scenario.rotations {
				t.Fatal("new generation was used to reconcile an old command", repository.completed, remote)
			}
		})
	}
}

type generationRevokeRemote struct {
	ProxyClient
	repository   *memberRevokeRepository
	changeAtRead int
	callErr      error
	reads        int
	rotations    int
}

func (r *generationRevokeRemote) DashboardForMember(context.Context, model.User) (RemoteDashboard, error) {
	r.reads++
	credential := "original-credential"
	if r.reads == r.changeAtRead {
		r.repository.user.AccountGeneration++
		credential = "replacement-account-credential"
	}
	return RemoteDashboard{SubscriptionURL: credential}, nil
}

func (r *generationRevokeRemote) RevokeSubscriptionForMember(context.Context, model.User) (string, error) {
	r.rotations++
	if errors.Is(r.callErr, database.ErrAccountUnavailable) {
		r.repository.user.AccountGeneration++
	}
	return "", r.callErr
}
