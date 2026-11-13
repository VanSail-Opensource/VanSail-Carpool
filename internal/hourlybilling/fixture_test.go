package hourlybilling

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type fixturePolicy struct {
	snapshot model.HourlyPolicySnapshot
	err      error
	reads    int
}

func (p *fixturePolicy) HourlyPolicy(context.Context) (model.HourlyPolicySnapshot, error) {
	p.reads++
	return p.snapshot, p.err
}

type fixtureNative struct {
	store           *database.Store
	mu              sync.Mutex
	resumes, closes int
	syncError       error
	readyHook       func(context.Context, string) (xnode.HourlyReadiness, error)
	preflightHook   func()
	syncHook        func(context.Context, string) error
}

func (*fixtureNative) CatalogTopology(context.Context) (xnode.CatalogTopology, error) {
	return xnode.CatalogTopology{}, nil
}
func (n *fixtureNative) HourlySquadReadiness(ctx context.Context, group string, _ ...string) (xnode.HourlyReadiness, error) {
	if n.readyHook != nil {
		return n.readyHook(ctx, group)
	}
	revision, _, err := n.store.XNodeCompileVersion(ctx, time.Now())
	at := time.Now().UTC()
	return xnode.HourlyReadiness{Ready: true, CompileRevision: revision, ObservedAt: at, ReadyUntil: at.Add(4 * time.Second)}, err
}
func (n *fixtureNative) PrepareHourlyPrincipal(ctx context.Context, id string) (xnode.HourlyPrincipal, error) {
	return n.store.EnsureXNodeHourlyPrincipal(ctx, id, "synthetic-encrypted-credentials")
}
func (n *fixtureNative) PreflightHourlyEnrollment(ctx context.Context, _ string) (xnode.HourlyReadiness, error) {
	if n.preflightHook != nil {
		n.preflightHook()
	}
	return n.HourlySquadReadiness(ctx, "hourly-group")
}
func (n *fixtureNative) SyncHourlyEnrollment(ctx context.Context, id string) error {
	if n.syncHook != nil {
		return n.syncHook(ctx, id)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.syncError
}
func (n *fixtureNative) ResumeHourlyOwner(context.Context, string, int64) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.resumes++
	return nil
}
func (n *fixtureNative) CloseHourlyOwnerIfIdle(context.Context, string, int64, string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.closes++
	return nil
}

func commerceFixture(t *testing.T) (*Service, model.User, *fixturePolicy, *fixtureNative) {
	t.Helper()
	ctx := context.Background()
	db, err := testdatabase.Open(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	store := database.NewStore(db)
	user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 97824, FirstName: "Hourly"}, false)
	if err != nil {
		t.Fatal(err)
	}
	actor, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 9_978_000_001, FirstName: "Hourly administrator"}, true)
	if err != nil {
		t.Fatal(err)
	}
	vault, err := secret.NewVault([]byte(strings.Repeat("h", 32)))
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.CompareAndSwapHourlyPolicy(ctx, actor.ID, "", "opaque-policy-fixture")
	if err != nil {
		t.Fatal(err)
	}
	policy := model.DefaultHourlyPolicy()
	policy.SalesEnabled, policy.ReducedRateBytesPerSecond, policy.ExtraGBPriceTXBMinor = true, 1024, 25
	reader := &fixturePolicy{snapshot: model.HourlyPolicySnapshot{Version: record.Version, Policy: policy}}
	if err = store.SaveXNodeGroup(ctx, xnode.Group{ID: "hourly-group", Name: "Hourly", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SaveHourlyOffer(ctx, actor.ID, model.HourlyOffer{GroupID: "hourly-group", Enabled: true, HourlyPriceTXBMinor: 100}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err = store.AdjustBalance(ctx, user.ID, 100000, "fixture", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	service := New(store, reader, nil, vault)
	native := &fixtureNative{store: store}
	service.native = native
	return service, user, reader, native
}

func queuedActivation(t *testing.T, s *Service, user model.User, policy *fixturePolicy) (providerops.Operation, model.OutboxJob) {
	t.Helper()
	receipt, err := s.Activate(context.Background(), user, ActivationRequest{GroupID: "hourly-group", PolicyVersion: policy.snapshot.Version, OfferRevision: 1}, "fixture-key")
	if err != nil {
		t.Fatal(err)
	}
	return reloadOperation(t, s, receipt.ID)
}
func reloadOperation(t *testing.T, s *Service, id string) (providerops.Operation, model.OutboxJob) {
	t.Helper()
	op, err := s.store.ProviderOperationByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var job model.OutboxJob
	job.Kind = providerops.OutboxKind
	if err = s.store.DB().QueryRow(`SELECT payload FROM outbox_jobs WHERE kind=$1 AND payload::jsonb->>'operationId'=$2`, providerops.OutboxKind, id).Scan(&job.Payload); err != nil {
		t.Fatal(err)
	}
	return op, job
}
func balanceFixture(t *testing.T, s *Service, user model.User) int64 {
	t.Helper()
	balance, err := s.store.Balance(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	return balance.MinorInt64()
}

var errNativeWait = errors.New("synthetic native result pending")
