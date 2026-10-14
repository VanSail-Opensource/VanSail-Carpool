package catalog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestCatalogAndPurchaseForwarding(t *testing.T) {
	t.Parallel()

	repository := &catalogRepository{
		combos:    []model.Combo{{ID: "combo-1"}},
		addons:    []model.SquadProduct{{ID: "addon-1", RemnaSquadUUID: "addon-squad"}},
		purchase:  model.Purchase{ID: "purchase-1"},
		purchases: []model.Purchase{{ID: "purchase-1"}},
	}
	service := newCatalogServiceForTest(repository, &catalogPurchaseRemote{catalogRemnawave: &catalogRemnawave{}})

	result, err := service.Catalog(context.Background())
	if err != nil {
		t.Fatalf("Catalog(): %v", err)
	}
	if len(result.Combos) != 1 || len(result.Addons) != 1 || !repository.combosVisibleOnly || repository.addonsVisibleOnly {
		t.Fatalf("Catalog() = %+v, filters = %t/%t", result, repository.combosVisibleOnly, repository.addonsVisibleOnly)
	}

	user := model.User{ID: "user-1", OnboardingState: "complete"}
	purchase, err := service.Purchase(context.Background(), user, "combo-1", []string{"addon-1"}, "purchase-attempt-1")
	if err != nil || purchase.ID != "purchase-1" {
		t.Fatalf("Purchase() = (%+v, %v)", purchase, err)
	}
	if repository.purchaseInput.UserID != user.ID || repository.purchaseInput.ComboID != "combo-1" || len(repository.purchaseInput.AddonSquadIDs) != 1 || repository.purchaseInput.IdempotencyKey != "purchase-attempt-1" {
		t.Fatalf("purchase input = %+v", repository.purchaseInput)
	}
	if !repository.purchaseAt.Equal(service.now().UTC()) {
		t.Fatalf("purchase time = %s, want %s", repository.purchaseAt, service.now().UTC())
	}

	history, err := service.Purchases(context.Background(), user.ID)
	if err != nil || len(history) != 1 || repository.listPurchasesUserID != user.ID {
		t.Fatalf("Purchases() = (%+v, %v), user %q", history, err, repository.listPurchasesUserID)
	}

	_, err = service.Purchase(context.Background(), model.User{ID: user.ID, OnboardingState: "agreement"}, "combo-1", nil, "purchase-attempt-2")
	if err == nil {
		t.Fatal("Purchase() before onboarding unexpectedly succeeded")
	}
	if _, err := service.Purchase(context.Background(), user, "", nil, "purchase-attempt-3"); err == nil {
		t.Fatal("Purchase() accepted an empty combo ID")
	}
	if _, err := service.Purchase(context.Background(), user, "combo-1", make([]string, 101), "purchase-attempt-4"); err == nil {
		t.Fatal("Purchase() accepted too many add-ons")
	}
	if _, err := service.Purchase(context.Background(), user, "combo-1", nil, ""); err == nil {
		t.Fatal("Purchase() accepted an empty idempotency key")
	}
}

func TestCatalogErrors(t *testing.T) {
	t.Parallel()

	testError := errors.New("repository failure")
	tests := []struct {
		name      string
		configure func(*catalogRepository)
	}{
		{name: "combos", configure: func(repository *catalogRepository) { repository.combosErr = testError }},
		{name: "addons", configure: func(repository *catalogRepository) { repository.addonsErr = testError }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &catalogRepository{}
			test.configure(repository)
			if _, err := newCatalogServiceForTest(repository, &catalogRemnawave{}).Catalog(context.Background()); err == nil {
				t.Fatal("Catalog() unexpectedly succeeded")
			}
		})
	}
}

func TestDashboardLocalFreshAndStale(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	lastReset := now.Add(-24 * time.Hour)
	remoteID := "remote-1"
	active := model.Purchase{ID: "active"}
	queued := model.Purchase{ID: "queued"}
	repository := &catalogRepository{balance: model.TXBMoney(1234), active: &active, queued: &queued}
	remnawave := &catalogRemnawave{dashboard: RemoteDashboard{
		Statistics:      model.Statistics{UsedTrafficBytes: "42", LastTrafficResetAt: &lastReset},
		SubscriptionURL: "https://subscription.test/token",
	}}
	service := NewService(repository, remnawave, time.Minute)
	service.now = func() time.Time { return now }

	local, err := service.Dashboard(context.Background(), model.User{ID: "local"})
	if err != nil {
		t.Fatalf("Dashboard(local): %v", err)
	}
	if local.Statistics != nil || local.Balance.Minor != "1234" || local.ActivePurchase == nil || local.QueuedPurchase == nil {
		t.Fatalf("local dashboard = %+v", local)
	}

	user := model.User{ID: "user-1", RemnaUserID: &remoteID}
	fresh, err := service.Dashboard(context.Background(), user)
	if err != nil {
		t.Fatalf("Dashboard(fresh): %v", err)
	}
	if fresh.Statistics == nil || fresh.Statistics.UsedTrafficBytes != "42" || fresh.SubscriptionURL == nil || fresh.StatisticsStale {
		t.Fatalf("fresh dashboard = %+v", fresh)
	}
	if remnawave.dashboardUserID != remoteID {
		t.Fatalf("remote dashboard user = %q, want %q", remnawave.dashboardUserID, remoteID)
	}
	if fresh.Statistics.LastTrafficResetAt == nil || !fresh.Statistics.LastTrafficResetAt.Equal(lastReset) {
		t.Fatal("upstream reset timestamp lost")
	}

	remnawave.dashboardErr = errors.New("upstream unavailable")
	service.now = func() time.Time { return now.Add(30 * time.Second) }
	cached, err := service.Dashboard(context.Background(), user)
	if err != nil {
		t.Fatalf("Dashboard(cached): %v", err)
	}
	if cached.StatisticsStale || cached.Statistics == nil || remnawave.dashboardCalls != 1 {
		t.Fatalf("cached dashboard = %+v, remote calls %d", cached, remnawave.dashboardCalls)
	}

	service.now = func() time.Time { return now.Add(2 * time.Minute) }
	stale, err := service.Dashboard(context.Background(), user)
	if err != nil {
		t.Fatalf("Dashboard(stale): %v", err)
	}
	if !stale.StatisticsStale || stale.Statistics == nil || stale.StatisticsWarning == "" || !stale.FetchedAt.Equal(now) {
		t.Fatalf("stale dashboard = %+v", stale)
	}
	if stale.Statistics.LastTrafficResetAt == nil || !stale.Statistics.LastTrafficResetAt.Equal(lastReset) {
		t.Fatal("cached reset timestamp lost")
	}
}

func TestDashboardErrors(t *testing.T) {
	t.Parallel()

	remoteID := "remote-1"
	testError := errors.New("failure")
	tests := []struct {
		name       string
		repository *catalogRepository
		remote     *catalogRemnawave
		user       model.User
	}{
		{name: "balance", repository: &catalogRepository{balanceErr: testError}, remote: &catalogRemnawave{}, user: model.User{ID: "user"}},
		{name: "entitlements", repository: &catalogRepository{activeErr: testError}, remote: &catalogRemnawave{}, user: model.User{ID: "user"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := newCatalogServiceForTest(test.repository, test.remote).Dashboard(context.Background(), test.user); err == nil {
				t.Fatal("Dashboard() unexpectedly succeeded")
			}
		})
	}

	service := newCatalogServiceForTest(&catalogRepository{}, &catalogRemnawave{dashboardErr: testError})
	uncached, err := service.Dashboard(context.Background(), model.User{ID: "user", RemnaUserID: &remoteID})
	if err != nil || !uncached.StatisticsStale || uncached.Statistics != nil || uncached.SubscriptionURL != nil || uncached.StatisticsWarning == "" {
		t.Fatalf("Dashboard(uncached upstream failure) = (%+v, %v)", uncached, err)
	}
}

func TestRevokeSubscription(t *testing.T) {
	t.Parallel()

	remoteID := "remote-1"
	testError := errors.New("failure")
	tests := []struct {
		name       string
		user       model.User
		repository *catalogRepository
		remote     *catalogRemnawave
		want       error
	}{
		{name: "missing remote identity", user: model.User{ID: "user"}, repository: &catalogRepository{}, remote: &catalogRemnawave{}, want: database.ErrNotFound},
		{name: "remote failure", user: model.User{ID: "user", RemnaUserID: &remoteID}, repository: &catalogRepository{}, remote: &catalogRemnawave{revokeErr: testError}, want: testError},
		{name: "success", user: model.User{ID: "user", RemnaUserID: &remoteID}, repository: &catalogRepository{}, remote: &catalogRemnawave{revokeURL: "https://new.test"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := newCatalogServiceForTest(test.repository, test.remote)
			service.cache[test.user.ID] = cachedDashboard{value: RemoteDashboard{SubscriptionURL: "https://old.test"}, fetchedAt: service.now()}
			got, err := service.RevokeSubscription(context.Background(), test.user)
			if test.want != nil {
				if !errors.Is(err, test.want) {
					t.Fatalf("RevokeSubscription() error = %v, want %v", err, test.want)
				}
				return
			}
			if _, exists := service.cache[test.user.ID]; err != nil || got != "https://new.test" || exists {
				t.Fatalf("RevokeSubscription() = (%q, %v), cache still present = %t", got, err, exists)
			}
		})
	}
}

type catalogRepository struct {
	combos              []model.Combo
	addons              []model.SquadProduct
	purchase            model.Purchase
	purchases           []model.Purchase
	balance             model.Money
	active              *model.Purchase
	queued              *model.Purchase
	combosErr           error
	addonsErr           error
	purchaseErr         error
	purchasesErr        error
	balanceErr          error
	activeErr           error
	combosVisibleOnly   bool
	addonsVisibleOnly   bool
	purchaseInput       database.PurchaseInput
	purchaseAt          time.Time
	listPurchasesUserID string
}

func (r *catalogRepository) ListCombos(_ context.Context, visible bool) ([]model.Combo, error) {
	r.combosVisibleOnly = visible
	return r.combos, r.combosErr
}
func (r *catalogRepository) ListSquadProducts(_ context.Context, visible bool) ([]model.SquadProduct, error) {
	r.addonsVisibleOnly = visible
	return r.addons, r.addonsErr
}
func (r *catalogRepository) CreatePurchase(_ context.Context, input database.PurchaseInput, at time.Time) (model.Purchase, error) {
	r.purchaseInput, r.purchaseAt = input, at
	return r.purchase, r.purchaseErr
}
func (r *catalogRepository) ListPurchases(_ context.Context, userID string) ([]model.Purchase, error) {
	r.listPurchasesUserID = userID
	return r.purchases, r.purchasesErr
}
func (r *catalogRepository) Balance(context.Context, string) (model.Money, error) {
	return r.balance, r.balanceErr
}
func (r *catalogRepository) ActiveAndQueuedPurchases(context.Context, string, time.Time) (*model.Purchase, *model.Purchase, error) {
	return r.active, r.queued, r.activeErr
}

type catalogRemnawave struct {
	dashboard       RemoteDashboard
	dashboardErr    error
	dashboardUserID string
	dashboardCalls  int
	revokeURL       string
	revokeErr       error
}

func (r *catalogRemnawave) Dashboard(_ context.Context, userID string) (RemoteDashboard, error) {
	r.dashboardCalls++
	r.dashboardUserID = userID
	return r.dashboard, r.dashboardErr
}
func (r *catalogRemnawave) RevokeSubscription(context.Context, string) (string, error) {
	return r.revokeURL, r.revokeErr
}

type catalogPurchaseRemote struct {
	*catalogRemnawave
}

func (*catalogPurchaseRemote) ListCatalogNodes(context.Context) ([]RemoteNode, error) {
	return []RemoteNode{{UUID: "node-1", Name: "Node", CountryCode: "US"}}, nil
}

func (*catalogPurchaseRemote) AccessibleCatalogNodeUUIDs(context.Context, string) ([]string, error) {
	return []string{"node-1"}, nil
}

func newCatalogServiceForTest(repository Repository, remnawave RemnawaveClient) *Service {
	service := NewService(repository, remnawave, time.Minute)
	service.now = func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) }
	return service
}
