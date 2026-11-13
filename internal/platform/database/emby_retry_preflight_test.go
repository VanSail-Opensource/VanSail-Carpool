package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestEmbyRetryReadsTargetBeforeConsumingAttempt(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "storage unavailable", err: errors.New("temporary database outage")},
		{name: "deadline", err: context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, store, repository, worker, operation, account := newEmbyRetryPreflight(t)
			repository.readErr = test.err
			if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{}); !errors.Is(err, test.err) {
				t.Fatalf("transient lookup lost its classification: %v", err)
			}
			unchanged, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
			if err != nil || unchanged.Receipt.Status != string(providerops.StatusQueued) || unchanged.Attempts != 0 {
				t.Fatalf("lookup outage consumed the operation: %+v, %v", unchanged, err)
			}
			items, err := store.ProviderOperationItems(ctx, operation.Receipt.ID)
			if err != nil || len(items) != 1 || items[0].Status != providerops.StatusQueued || items[0].AttemptStartedAt != nil {
				t.Fatalf("lookup outage consumed the item: %+v, %v", items, err)
			}
			repository.readErr = nil
			now := time.Now().UTC()
			if _, err := store.BeginEmbyProvisioning(ctx, account.ID, now); err != nil {
				t.Fatal(err)
			}
			if err := store.SetEmbyRemoteIdentity(ctx, account.ID, "remote", account.BaseUsername, now); err != nil {
				t.Fatal(err)
			}
			if err := store.MarkEmbyProvisioned(ctx, account.ID, domain.Preferences{}, now); err != nil {
				t.Fatal(err)
			}
			if err := worker.HandleProviderOperation(ctx, unchanged, model.OutboxJob{}); err != nil {
				t.Fatal(err)
			}
			completed, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
			if err != nil || completed.Receipt.Status != string(providerops.StatusSucceeded) {
				t.Fatalf("retry did not recover: %+v, %v", completed, err)
			}
		})
	}
}

func TestEmbyRetryConfirmedMissingTargetIsRejected(t *testing.T) {
	t.Parallel()
	ctx, store, repository, worker, operation, _ := newEmbyRetryPreflight(t)
	repository.readErr = fmt.Errorf("read reviewed retry: %w", domain.ErrNotFound)
	if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	completed, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
	if err != nil || completed.Receipt.Status != string(providerops.StatusFailed) ||
		completed.Receipt.ErrorCode == nil || *completed.Receipt.ErrorCode != "EMBY_RETRY_REJECTED" {
		t.Fatalf("confirmed missing target was not classified: %+v, %v", completed, err)
	}
}

func newEmbyRetryPreflight(t *testing.T) (context.Context, *Store, *embyRetryReadStore, *domain.OperationService, providerops.Operation, domain.Account) {
	t.Helper()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestAdmin(t, store, 98_920)
	owner := createTestUser(t, store, 98_921)
	account, _, err := store.QueueEmbySetup(ctx, domain.QueueSetupInput{
		ID: "retry-read", UserID: owner.ID, BaseUsername: owner.AccountName(),
		PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:" + owner.ID,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	repository := &embyRetryReadStore{Store: store}
	worker, err := domain.NewOperationService(domain.NewService(repository, nil, nil, nil), store,
		&embyReplaySecrets{reject: true}, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := worker.QueueProvisionRetry(ctx, actor.ID, account.ID, "retry-read-command")
	if err != nil {
		t.Fatal(err)
	}
	operation, err := store.ProviderOperationByID(ctx, receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, store, repository, worker, operation, account
}

type embyRetryReadStore struct {
	*Store
	readErr error
}

func (s *embyRetryReadStore) EmbyProvisioningByID(ctx context.Context, id string) (domain.ProvisioningRecord, error) {
	if s.readErr != nil {
		return domain.ProvisioningRecord{}, s.readErr
	}
	return s.Store.EmbyProvisioningByID(ctx, id)
}
