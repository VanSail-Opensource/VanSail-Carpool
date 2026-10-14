package admin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestUserProviderWorkerRelinksOnlyVerifiedTarget(t *testing.T) {
	t.Parallel()
	repository := &adminUserProviderRepositoryStub{desired: &model.Purchase{TrafficLimitBytes: 1_024, ResetStrategy: "MONTH_ROLLING", SquadUUIDs: []string{"squad"}, ValidUntil: time.Now().Add(time.Hour)}}
	remote := &adminUserProviderRemoteStub{}
	worker := NewUserProviderWorker(repository, remote)
	worker.now = func() time.Time { return time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC) }
	target, _ := json.Marshal(map[string]string{"remoteId": "220", "reason": "upstream correction"})
	completion := worker.run(context.Background(), providerops.Operation{ActorUserID: "admin", OwnerUserID: "member",
		Receipt: model.OperationReceipt{Kind: providerops.KindAdminRemnaRelink}}, model.OutboxJob{Kind: providerops.OutboxKind, Payload: `{"sealedTarget":` + strconvQuote(string(target)) + `}`})
	if completion.Status != providerops.StatusSucceeded || repository.linkedRemoteID != "220" || remote.verifiedID != "220" || remote.appliedID != "220" {
		t.Fatalf("relink outcome = %+v, repository=%+v, remote=%+v", completion, repository, remote)
	}
	if repository.userCalls != 0 || remote.disabledID != "" {
		t.Fatalf("former remote account was consulted or mutated: repository=%+v remote=%+v", repository, remote)
	}
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

type adminUserProviderRepositoryStub struct {
	desired        *model.Purchase
	linkedRemoteID string
	userCalls      int
}

func (*adminUserProviderRepositoryStub) BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error) {
	return providerops.Operation{}, errors.New("unexpected")
}
func (*adminUserProviderRepositoryStub) CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error) {
	return providerops.Operation{}, errors.New("unexpected")
}
func (r *adminUserProviderRepositoryStub) UserByID(context.Context, string) (model.User, error) {
	r.userCalls++
	return model.User{}, errors.New("unexpected")
}
func (r *adminUserProviderRepositoryStub) DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	return r.desired, nil
}
func (r *adminUserProviderRepositoryStub) LinkAdminRemnaUser(_ context.Context, _, _, remoteID, _ string, _ time.Time) error {
	r.linkedRemoteID = remoteID
	return nil
}
func (*adminUserProviderRepositoryStub) HasActiveAbuseTemporaryBan(context.Context, string, time.Time) (bool, error) {
	return false, nil
}
func (*adminUserProviderRepositoryStub) CompleteAdminTemporaryUnban(context.Context, string, time.Time) error {
	return nil
}

type adminUserProviderRemoteStub struct{ verifiedID, appliedID, disabledID string }

func (r *adminUserProviderRemoteStub) SetAdminUserDisabled(_ context.Context, remoteID string, _ bool) error {
	r.disabledID = remoteID
	return nil
}
func (r *adminUserProviderRemoteStub) VerifyAdminRemoteUser(_ context.Context, remoteID string) error {
	r.verifiedID = remoteID
	return nil
}
func (r *adminUserProviderRemoteStub) ApplyEntitlement(_ context.Context, remoteID string, _ int64, _ string, _ []string, _ time.Time) error {
	r.appliedID = remoteID
	return nil
}
func (*adminUserProviderRemoteStub) RemoveEntitlement(context.Context, string) error { return nil }
