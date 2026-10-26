package catalog

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type automaticRenewalAddonSelectionRepository interface {
	AutoRenewalPlanExcludingAddons(context.Context, string, string, []string, time.Time) (database.AutoRenewalPlan, error)
	CommitAutoRenewalExcludingAddons(context.Context, string, []string, time.Time) (model.Purchase, error)
}

func (s *Service) automaticRenewalSelectionAt(ctx context.Context, user model.User, purchaseID string, now time.Time) (model.AutoRenewal, []string, error) {
	repository, ok := s.repository.(automaticRenewalRepository)
	if !ok || strings.TrimSpace(user.ID) == "" || strings.TrimSpace(purchaseID) == "" {
		return model.AutoRenewal{}, nil, database.ErrNotFound
	}
	plan, err := repository.AutoRenewalPlan(ctx, user.ID, purchaseID, now)
	if err != nil {
		return model.AutoRenewal{}, nil, err
	}
	result := autoRenewalFromPlan(plan)
	if plan.IneligibleReason != "" {
		result.CanEnable = false
		return result, nil, nil
	}
	addonIDs := renewalAddonIDs(plan.Addons)
	catalog, unavailable, reason, err := s.hydrateRenewalCatalog(ctx, []model.Combo{plan.Combo}, addonIDs)
	if err != nil {
		return model.AutoRenewal{}, nil, err
	}
	if reason != "" {
		result.IneligibleReason = autoRenewalReason(reason)
		return result, nil, nil
	}
	if len(unavailable) > 0 {
		selection, ok := repository.(automaticRenewalAddonSelectionRepository)
		if !ok {
			return model.AutoRenewal{}, nil, errors.New("automatic renewal selection is unavailable")
		}
		plan, err = selection.AutoRenewalPlanExcludingAddons(ctx, user.ID, purchaseID, unavailable, now)
		if err != nil {
			return model.AutoRenewal{}, nil, err
		}
		result = autoRenewalFromPlan(plan)
		if plan.IneligibleReason != "" {
			result.CanEnable = false
			return result, unavailable, nil
		}
	}
	result.IneligibleReason, err = s.autoRenewalLiveReason(ctx, plan, catalog)
	if err != nil {
		return model.AutoRenewal{}, nil, err
	}
	result.CanEnable = result.IneligibleReason == nil
	return result, unavailable, nil
}

func (s *Service) commitAutomaticRenewal(ctx context.Context, repository automaticRenewalRepository, purchaseID string, unavailable []string, now time.Time) (model.Purchase, error) {
	if len(unavailable) == 0 {
		return repository.CommitAutoRenewal(ctx, purchaseID, now)
	}
	selection, ok := repository.(automaticRenewalAddonSelectionRepository)
	if !ok {
		return model.Purchase{}, errors.New("automatic renewal selection is unavailable")
	}
	return selection.CommitAutoRenewalExcludingAddons(ctx, purchaseID, unavailable, now)
}

func renewalAddonIDs(addons []model.SquadProduct) []string {
	ids := make([]string, 0, len(addons))
	for _, addon := range addons {
		ids = append(ids, addon.GroupID)
	}
	return ids
}
