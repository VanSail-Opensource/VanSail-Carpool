package database

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestEmbyPasswordParentWriteFailurePreservesKnownSuccess(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 98_900)
	now := time.Now().UTC()
	account, _, err := store.QueueEmbySetup(ctx, domain.QueueSetupInput{
		ID: "replay-account", UserID: owner.ID, BaseUsername: owner.AccountName(),
		PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:" + owner.ID,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginEmbyProvisioning(ctx, account.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEmbyRemoteIdentity(ctx, account.ID, "remote-user", owner.AccountName(), now); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkEmbyProvisioned(ctx, account.ID, domain.Preferences{}, now); err != nil {
		t.Fatal(err)
	}
	repository := &embyReplayStore{Store: store, fail: errors.New("receipt temporarily unavailable")}
	secrets := &embyReplaySecrets{}
	remote := &embyReplayRemote{}
	core := domain.NewService(store, remote, nil, nil)
	worker, err := domain.NewOperationService(core, repository, secrets, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := worker.QueuePassword(ctx, owner.ID, "replacement-password", "password-replay")
	if err != nil {
		t.Fatal(err)
	}
	var payload string
	if err := store.DB().QueryRowContext(ctx, `SELECT payload FROM outbox_jobs WHERE kind=$1 AND payload::jsonb->>'operationId'=$2`, providerops.OutboxKind, receipt.ID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	job := model.OutboxJob{Payload: payload}
	operation, err := store.ProviderOperationByID(ctx, receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleProviderOperation(ctx, operation, job); !errors.Is(err, repository.fail) {
		t.Fatalf("expected interrupted parent receipt: %v", err)
	}
	items, err := store.ProviderOperationItems(ctx, receipt.ID)
	if err != nil || len(items) != 1 || items[0].Status != providerops.StatusSucceeded {
		t.Fatalf("successful password outcome not retained: %+v, %v", items, err)
	}
	repository.fail, secrets.reject = nil, true
	operation, err = store.ProviderOperationByID(ctx, receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleProviderOperation(ctx, operation, job); err != nil {
		t.Fatal(err)
	}
	completed, err := store.ProviderOperationByID(ctx, receipt.ID)
	if err != nil || completed.Receipt.Status != string(providerops.StatusSucceeded) ||
		completed.ResultJSON != items[0].ResultJSON || remote.calls != 1 || secrets.opens != 1 {
		t.Fatalf("known success was lost or repeated: %+v, %v, writes=%d decryptions=%d", completed, err, remote.calls, secrets.opens)
	}
	if err := worker.HandleProviderOperation(ctx, completed, model.OutboxJob{}); err != nil || remote.calls != 1 || secrets.opens != 1 {
		t.Fatal("terminal parent replay used retired state", err)
	}
}

func TestEmbyRetainedItemRepairsExactOutcomeWithoutLiveDependencies(t *testing.T) {
	for _, status := range []providerops.Status{providerops.StatusSucceeded, providerops.StatusFailed, providerops.StatusPendingReview} {
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			owner := createTestUser(t, store, 98_901)
			now := time.Now().UTC()
			operation, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{
				ActorUserID: owner.ID, OwnerUserID: owner.ID, Kind: providerops.KindEmbyPreferences,
				IdempotencyKey: "retained-item", RequestFingerprint: "retained-item-fingerprint",
				Items: []providerops.ItemInput{{Key: "emby", TargetType: "emby_user", TargetID: owner.ID}},
			}, now)
			if err != nil {
				t.Fatal(err)
			}
			operation, err = store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "emby", now); err != nil {
				t.Fatal(err)
			}
			code := ""
			if status != providerops.StatusSucceeded {
				code = "ORIGINAL_EMBY_OUTCOME"
			}
			item, err := store.CompleteProviderOperationItem(ctx, operation.Receipt.ID, "emby", providerops.Completion{
				Status: status, ErrorCode: code, ProviderReference: "remote-proof", ResultJSON: `{"proof":"recorded"}`,
			}, now)
			if err != nil {
				t.Fatal(err)
			}
			secrets := &embyReplaySecrets{reject: true}
			worker, err := domain.NewOperationService(domain.NewService(nil, nil, nil, nil), store, secrets, make([]byte, 32))
			if err != nil {
				t.Fatal(err)
			}
			if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{}); err != nil {
				t.Fatal(err)
			}
			got, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
			if err != nil || got.Receipt.Status != string(status) || got.ResultJSON != item.ResultJSON ||
				got.ProviderReference != item.ProviderReference || secrets.opens != 0 {
				t.Fatalf("retained evidence changed: %+v, %v", got, err)
			}
			gotCode := ""
			if got.Receipt.ErrorCode != nil {
				gotCode = *got.Receipt.ErrorCode
			}
			if gotCode != code {
				t.Fatalf("retained classification changed: %s -> %s", code, gotCode)
			}
		})
	}
}

type embyReplayStore struct {
	*Store
	fail error
}

func (s *embyReplayStore) CompleteProviderOperation(ctx context.Context, id string, completion providerops.Completion, now time.Time) (providerops.Operation, error) {
	if s.fail != nil {
		return providerops.Operation{}, s.fail
	}
	return s.Store.CompleteProviderOperation(ctx, id, completion, now)
}

type embyReplaySecrets struct {
	payload []byte
	reject  bool
	opens   int
}

func (s *embyReplaySecrets) Seal(_ string, payload []byte) (string, error) {
	s.payload = append([]byte(nil), payload...)
	return "sealed", nil
}

func (s *embyReplaySecrets) Open(string, string) ([]byte, error) {
	s.opens++
	if s.reject {
		return nil, errors.New("retired encryption key must not be consulted")
	}
	return append([]byte(nil), s.payload...), nil
}

type embyReplayRemote struct {
	domain.Remote
	calls int
}

func (r *embyReplayRemote) SetPassword(context.Context, string, []byte, []byte) error {
	r.calls++
	return nil
}

func (*embyReplayRemote) IsTerminal(error) bool { return false }
