package catalog

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// renewalCatalog hydrates only an owned renewal selection. Retained add-ons
// bypass storefront visibility, while unavailable paid squads are excluded.
// The provider adapter keeps all live reads behind the existing queue.
func (s *Service) renewalCatalog(ctx context.Context, comboID string, addonIDs []string) (model.Catalog, []string, string, error) {
	combos, err := s.repository.ListCombos(ctx, true)
	if err != nil {
		return model.Catalog{}, nil, "", err
	}
	selectedCombos := make([]model.Combo, 0, 1)
	for _, combo := range combos {
		if combo.ID == comboID {
			selectedCombos = append(selectedCombos, combo)
			break
		}
	}
	return s.hydrateRenewalCatalog(ctx, selectedCombos, addonIDs)
}

// hydrateRenewalCatalog validates the selected entitlement, including persisted
// custom squads, through the same queued provider used by the storefront.
func (s *Service) hydrateRenewalCatalog(ctx context.Context, combos []model.Combo, addonIDs []string) (model.Catalog, []string, string, error) {
	addons := make([]model.SquadProduct, 0, len(addonIDs))
	for _, id := range addonIDs {
		addons = append(addons, model.SquadProduct{ID: id, RemnaSquadUUID: id, Visible: true})
	}
	catalog, err := s.hydrateLiveCatalog(ctx, combos, addons)
	if err != nil {
		return model.Catalog{}, nil, "", err
	}
	if len(catalog.Combos) == 0 {
		return catalog, nil, database.AutoRenewalReasonComboUnavailable, nil
	}
	liveAddons := make(map[string]bool, len(catalog.Addons))
	for _, addon := range catalog.Addons {
		liveAddons[addon.RemnaSquadUUID] = true
	}
	unavailable := make([]string, 0)
	for _, id := range addonIDs {
		if !liveAddons[id] {
			unavailable = append(unavailable, id)
		}
	}
	return catalog, unavailable, "", nil
}
