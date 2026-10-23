package catalog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestCatalogAndPurchaseRejectMissingLiveSquads(t *testing.T) {
	t.Parallel()

	repository := &catalogRepository{
		combos: []model.Combo{
			{ID: "live-combo", IncludedSquads: []model.SquadProduct{{GroupID: "live-squad"}}},
			{ID: "stale-combo", IncludedSquads: []model.SquadProduct{{GroupID: "stale-squad"}}},
		},
		addons: []model.SquadProduct{
			{ID: "live-addon", GroupID: "live-addon", Visible: true},
			{ID: "stale-addon", GroupID: "stale-addon", Visible: true},
		},
		purchase: model.Purchase{ID: "purchase-1"},
	}
	remote := &catalogLiveRemote{
		catalogProxy: &catalogProxy{},
		squads:       []RemoteSquad{{UUID: "live-squad", Name: "Core"}, {UUID: "live-addon", Name: "Extra"}},
	}
	service := NewService(repository, remote, time.Minute)
	user := model.User{ID: "user-1", OnboardingState: "complete"}

	catalog, err := service.Catalog(context.Background())
	if err != nil || len(catalog.Combos) != 1 || catalog.Combos[0].ID != "live-combo" || len(catalog.Addons) != 1 || catalog.Addons[0].Name != "Extra" {
		t.Fatalf("Catalog() = (%+v, %v)", catalog, err)
	}
	if _, err := service.Purchase(context.Background(), user, "stale-combo", nil, "stale-combo-attempt"); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("Purchase(stale combo) = %v, want ErrNotFound", err)
	}
	if _, err := service.Purchase(context.Background(), user, "live-combo", []string{"stale-addon"}, "stale-addon-attempt"); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("Purchase(stale addon) = %v, want ErrNotFound", err)
	}
	if _, err := service.Purchase(context.Background(), user, "live-combo", []string{"live-addon"}, "live-attempt"); !errors.Is(err, ErrNoAccessibleNodes) {
		t.Fatalf("Purchase(live selection) = %v, want ErrNoAccessibleNodes", err)
	}
}

type catalogLiveRemote struct {
	*catalogProxy
	squads []RemoteSquad
	err    error
}

func (r *catalogLiveRemote) ListCatalogSquads(context.Context) ([]RemoteSquad, error) {
	return r.squads, r.err
}
