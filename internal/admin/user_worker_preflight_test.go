package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestAdminEntitlementReadOutageDoesNotClaimItemAttempt(t *testing.T) {
	for _, source := range []string{"owner", "desired entitlement"} {
		t.Run(source, func(t *testing.T) {
			repository := newUserOperationRepositoryStub([]string{"owner"})
			repository.users["owner"] = model.User{ID: "owner"}
			repository.desired["owner"] = desiredAdminPurchase()
			unavailable := errors.New("temporary storage failure")
			if source == "owner" {
				repository.userErr = unavailable
			} else {
				repository.desiredErr = unavailable
			}
			remote := &entitlementRemoteStub{}
			worker := NewUserOperationWorker(repository, remote)
			if err := worker.HandleProviderOperation(t.Context(), repository.operation, model.OutboxJob{}); !errors.Is(err, unavailable) {
				t.Fatal("storage outage became a recorded provider failure", err)
			}
			if repository.items[0].Status != providerops.StatusQueued || repository.items[0].AttemptStartedAt != nil ||
				repository.operation.Receipt.CompletedAt != nil || len(remote.applyCalls) != 0 {
				t.Fatal("preflight outage claimed mutation or completion", repository.operation, repository.items)
			}
			repository.userErr, repository.desiredErr = nil, nil
			if err := worker.HandleProviderOperation(t.Context(), repository.operation, model.OutboxJob{}); err != nil {
				t.Fatal(err)
			}
			if repository.operation.Receipt.Status != string(providerops.StatusSucceeded) || len(remote.applyCalls) != 1 {
				t.Fatal("recovered preflight could not make its first mutation", repository.operation, remote.applyCalls)
			}
		})
	}
}

func TestAdminRestrictionOwnerReadOutageRemainsRetryable(t *testing.T) {
	for _, kind := range []string{providerops.KindAdminTemporaryBan, providerops.KindAdminTemporaryUnban} {
		for _, phase := range []providerops.Status{providerops.StatusQueued, providerops.StatusProcessing} {
			t.Run(kind+"/"+string(phase), func(t *testing.T) {
				repository := &restrictionPreflightRepository{userOperationRepositoryStub: newUserOperationRepositoryStub(nil)}
				repository.operation.OwnerUserID = "owner"
				repository.operation.Receipt.Kind, repository.operation.Receipt.Status = kind, string(phase)
				repository.users["owner"] = model.User{ID: "owner"}
				repository.userErr = context.DeadlineExceeded
				remote := &restrictionPreflightRemote{}
				worker := NewUserProviderWorker(repository, remote)
				if err := worker.HandleProviderOperation(t.Context(), repository.operation, model.OutboxJob{}); !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("local read timeout became a terminal operation", err)
				}
				if remote.calls != 0 || repository.unbans != 0 || repository.operation.Receipt.CompletedAt != nil {
					t.Fatal("read timeout changed a restriction or receipt", repository.operation)
				}
				repository.userErr = nil
				if err := worker.HandleProviderOperation(t.Context(), repository.operation, model.OutboxJob{}); err != nil {
					t.Fatal(err)
				}
				if repository.operation.Receipt.Status != string(providerops.StatusSucceeded) || remote.calls != 1 ||
					remote.disabled != (kind == providerops.KindAdminTemporaryBan) || (repository.unbans == 1) != (kind == providerops.KindAdminTemporaryUnban) {
					t.Fatal("retry did not finish the admitted restriction", repository.operation, remote)
				}
			})
		}
	}
}

type restrictionPreflightRepository struct {
	*userOperationRepositoryStub
	unbans int
}

func (r *restrictionPreflightRepository) CompleteAdminTemporaryUnban(context.Context, string, string, time.Time) error {
	r.unbans++
	return nil
}

type restrictionPreflightRemote struct {
	calls    int
	disabled bool
}

func (r *restrictionPreflightRemote) SetAdminUserDisabled(_ context.Context, _ model.User, _ string, disabled bool) error {
	r.calls++
	r.disabled = disabled
	return nil
}
