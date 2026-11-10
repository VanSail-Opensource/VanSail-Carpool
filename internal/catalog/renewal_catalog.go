package catalog

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// renewalComboRepository resolves the combo that one owned renewal provisions.
type renewalComboRepository interface {
	RenewalCombo(context.Context, string, string) (model.Combo, error)
}

// renewalCatalog hydrates only an owned renewal selection. The store resolves
// the purchase's own combo, so a hidden or inactive combo row stays renewable and
// the recorded purchase bundle keeps precedence over the live one. Hydration then
// validates the same owned selection the store commits: retained add-ons bypass
// storefront visibility, while unavailable paid squads are excluded. The provider
// adapter keeps all live reads behind the existing queue.
func (s *Service) renewalCatalog(ctx context.Context, userID, purchaseID, comboID string, addonIDs []string) (model.Catalog, []string, string, error) {
	resolver, ok := s.repository.(renewalComboRepository)
	if !ok {
		return model.Catalog{}, nil, "", errors.New("renewal selection is unavailable")
	}
	combo, err := resolver.RenewalCombo(ctx, userID, purchaseID)
	if err != nil {
		return model.Catalog{}, nil, "", err
	}
	// A concurrent combo replacement between the quote and this resolution must
	// not hydrate a different product than the quote priced.
	if combo.ID != comboID {
		return model.Catalog{}, nil, "", database.ErrNotFound
	}
	return s.hydrateRenewalCatalog(ctx, []model.Combo{combo}, addonIDs)
}

// hydrateRenewalCatalog validates the selected entitlement, including persisted
// custom squads, through the same queued provider used by the storefront.
// Billing mode is merchandising state: every purchased included squad and add-on
// stays in the owned selection even if the product was later re-marked PAYG. Only
// missing live upstream identities or unavailable nodes remove a retained squad.
func (s *Service) hydrateRenewalCatalog(ctx context.Context, combos []model.Combo, addonIDs []string) (model.Catalog, []string, string, error) {
	addons := make([]model.SquadProduct, 0, len(addonIDs))
	for _, id := range addonIDs {
		addons = append(addons, model.SquadProduct{ID: id, GroupID: id, Visible: true})
	}
	catalog, err := s.hydrateOwnedCatalog(ctx, combos, addons)
	if err != nil {
		return model.Catalog{}, nil, "", err
	}
	if len(catalog.Combos) == 0 {
		return catalog, nil, database.AutoRenewalReasonComboUnavailable, nil
	}
	liveAddons := make(map[string]bool, len(catalog.Addons))
	for _, addon := range catalog.Addons {
		liveAddons[addon.GroupID] = true
	}
	unavailable := make([]string, 0)
	for _, id := range addonIDs {
		if !liveAddons[id] {
			unavailable = append(unavailable, id)
		}
	}
	return catalog, unavailable, "", nil
}
