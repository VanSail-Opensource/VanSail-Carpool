package catalog

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CatalogForUser returns the displayable catalog for an onboarded member.
// Checkout operations enforce automatic-renewal restrictions separately.
func (s *Service) CatalogForUser(ctx context.Context, user model.User) (model.Catalog, error) {
	catalog, err := s.Catalog(ctx)
	if err != nil {
		return model.Catalog{}, err
	}
	if admission, ok := s.repository.(interface {
		AutoRenewalBlockedUntil(context.Context, string, time.Time) (*time.Time, error)
	}); ok {
		catalog.AutoRenewalBlockedUntil, err = admission.AutoRenewalBlockedUntil(ctx, user.ID, s.now().UTC())
		if err != nil {
			return model.Catalog{}, err
		}
	}
	active, _, err := s.repository.ActiveAndQueuedPurchases(ctx, user.ID, s.now().UTC())
	if err != nil || active == nil {
		return catalog, err
	}
	held := make(map[string]struct{}, len(active.SquadUUIDs))
	for _, squadUUID := range active.SquadUUIDs {
		held[squadUUID] = struct{}{}
	}
	for index := range catalog.Addons {
		_, catalog.Addons[index].StockHeldByCurrentUser = held[catalog.Addons[index].GroupID]
	}
	for comboIndex := range catalog.Combos {
		for squadIndex := range catalog.Combos[comboIndex].IncludedSquads {
			_, catalog.Combos[comboIndex].IncludedSquads[squadIndex].StockHeldByCurrentUser = held[catalog.Combos[comboIndex].IncludedSquads[squadIndex].GroupID]
		}
	}
	return catalog, nil
}
