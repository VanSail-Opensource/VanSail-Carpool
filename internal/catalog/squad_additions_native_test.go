package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type liveAddonRemote struct {
	*catalogProxy
	groups []RemoteSquad
}

func (r *liveAddonRemote) ListCatalogSquads(context.Context) ([]RemoteSquad, error) {
	return r.groups, nil
}

func TestAddonValidationUsesLiveNativeGroups(t *testing.T) {
	repo := newPurchaseAddonCatalogRepository()
	remote := &liveAddonRemote{catalogProxy: &catalogProxy{}, groups: []RemoteSquad{{UUID: "squad-1", Name: "Native"}}}
	service := newCatalogServiceForTest(repo, remote)
	user := model.User{ID: "member", OnboardingState: "complete"}
	if _, err := service.QuoteAddons(context.Background(), user, "purchase", []string{"squad-1"}); err != nil {
		t.Fatal(err)
	}
	remote.groups = nil
	if _, err := service.QuoteAddons(context.Background(), user, "purchase", []string{"squad-1"}); !errors.Is(err, database.ErrNotFound) {
		t.Fatal("removed native group remained purchasable", err)
	}
}

type addonReplayRepository struct {
	*purchaseAddonCatalogRepository
	owner string
}

func (r *addonReplayRepository) PurchaseAddonReplay(_ context.Context, input database.PurchaseAddonInput) (model.Purchase, bool, error) {
	r.owner = input.UserID
	return model.Purchase{ID: input.PurchaseID}, true, nil
}
func TestAddonServiceReplaysBeforeReadingChangedLiveCatalog(t *testing.T) {
	repo := &addonReplayRepository{purchaseAddonCatalogRepository: newPurchaseAddonCatalogRepository()}
	repo.addonsErr = errors.New("live catalog must not be read on replay")
	service := newCatalogServiceForTest(repo, &catalogProxy{})
	purchase, err := service.AddAddons(context.Background(), model.User{ID: "member", OnboardingState: "complete"}, "purchase", []string{"squad-1"}, nil, "retry")
	if err != nil || purchase.ID != "purchase" || repo.owner != "member" {
		t.Fatal(purchase, err, repo.owner)
	}
}
