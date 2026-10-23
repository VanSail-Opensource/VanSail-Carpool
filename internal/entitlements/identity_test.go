package entitlements

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type identityRepositoryStub struct {
	*entitlementRepository
	purchased     bool
	linkedID      string
	conflictCalls int
}

func (r *identityRepositoryStub) HasProvisionablePurchase(context.Context, string, time.Time) (bool, error) {
	return r.purchased, nil
}
func (r *identityRepositoryStub) LinkProvisionedProxyUser(_ context.Context, _, _, _, remoteID string, _ time.Time) error {
	r.linkedID = remoteID
	r.user.ProxyUserID = &remoteID
	return nil
}
func (r *identityRepositoryStub) ResolveProvisioningConflict(context.Context, string, string, time.Time) error {
	r.conflictCalls++
	return nil
}
func (r *identityRepositoryStub) QueueProxyRepair(_ context.Context, _, _ string, _ time.Time) (model.User, error) {
	r.user.ProxyUserID = nil
	return r.user, nil
}

type identityClientStub struct {
	byID, byName, byTelegram       accounts.RemoteUser
	findID, findName, findTelegram bool
	findErr                        error
	created                        accounts.RemoteUser
	createInput                    accounts.RemoteCreateUser
	createCalls                    int
}

func (r *identityClientStub) FindUserByID(context.Context, string) (accounts.RemoteUser, bool, error) {
	return r.byID, r.findID, r.findErr
}
func (r *identityClientStub) FindUserByUsername(context.Context, string) (accounts.RemoteUser, bool, error) {
	return r.byName, r.findName, nil
}
func (r *identityClientStub) FindUserByTelegramID(context.Context, int64) (accounts.RemoteUser, bool, error) {
	return r.byTelegram, r.findTelegram, nil
}
func (r *identityClientStub) CreateUser(_ context.Context, input accounts.RemoteCreateUser) (accounts.RemoteUser, error) {
	r.createCalls++
	r.createInput = input
	return r.created, nil
}
func (r *identityClientStub) IsDuplicateError(error) bool { return false }

func TestPurchaseProvisioningAndRecovery(t *testing.T) {
	t.Parallel()
	username, oldID := "reserved", "old"
	telegramID, otherTelegram := int64(42), int64(99)
	created := accounts.RemoteUser{ID: "101", Username: username, TelegramID: &telegramID}
	outage := errors.New("provider unavailable")
	for _, test := range []struct {
		name         string
		linkedID     *string
		desired      *model.Purchase
		identity     identityClientStub
		wantCreate   int
		wantApply    int
		wantConflict int
		wantError    error
	}{
		{name: "first queued purchase creates disabled user", identity: identityClientStub{created: created}, wantCreate: 1},
		{name: "missing active user recreates same username", linkedID: &oldID,
			desired:  &model.Purchase{TrafficLimitBytes: 1000, ResetStrategy: "DAY", ValidUntil: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
			identity: identityClientStub{created: created}, wantCreate: 1, wantApply: 1},
		{name: "username taken by another account refunds", identity: identityClientStub{
			byName: accounts.RemoteUser{ID: "other", Username: username, TelegramID: &otherTelegram}, findName: true,
		}, wantConflict: 1},
		{name: "missing Telegram identity cannot authorize a refund", identity: identityClientStub{
			byName: accounts.RemoteUser{ID: "102", Username: username}, findName: true,
		}, wantError: errIdentityUnverified},
		{name: "provider outage stays retryable", linkedID: &oldID, identity: identityClientStub{findErr: outage}, wantError: outage},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := &identityRepositoryStub{entitlementRepository: &entitlementRepository{
				user:    model.User{ID: "user", TelegramID: telegramID, Username: &username, ProxyUserID: test.linkedID, OnboardingState: "complete"},
				desired: test.desired,
			}, purchased: true}
			remote := &entitlementProxy{}
			identity := test.identity
			worker := NewWorker(repo, remote, &identity)
			worker.now = func() time.Time { return time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC) }
			err := worker.HandleOutbox(context.Background(), model.OutboxJob{Kind: "xnode_sync_user", Payload: `{"userId":"user"}`})
			if test.wantError != nil && !errors.Is(err, test.wantError) {
				t.Fatalf("HandleOutbox() = %v, want %v", err, test.wantError)
			}
			if test.wantError == nil && err != nil {
				t.Fatal(err)
			}
			if identity.createCalls != test.wantCreate || remote.applyCalls != test.wantApply || repo.conflictCalls != test.wantConflict {
				t.Fatalf("create/apply/conflict = %d/%d/%d, want %d/%d/%d", identity.createCalls, remote.applyCalls, repo.conflictCalls,
					test.wantCreate, test.wantApply, test.wantConflict)
			}
			if test.wantCreate > 0 && (identity.createInput.Status != "DISABLED" || identity.createInput.Username != repo.user.AccountName() || repo.linkedID != "101") {
				t.Fatalf("provisioned input/link = %+v/%q", identity.createInput, repo.linkedID)
			}
		})
	}
}
