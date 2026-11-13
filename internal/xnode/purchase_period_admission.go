package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

type purchasePeriodAdmissionRepository interface {
	ActivateXNodePurchasePeriod(context.Context, Identity, Period, model.Purchase, string, int64, time.Time) (Period, error)
}

// Target admission and predecessor fencing commit together. Network barriers
// continue through the existing accounting recovery without delaying paid access.
func (s *Service) admitPurchasePeriod(ctx context.Context, repository purchasePeriodAdmissionRepository, identity Identity, previous Period, purchase model.Purchase, generation int64) error {
	generation, err := s.purchasePeriodGeneration(ctx, identity, generation)
	if err != nil {
		return err
	}
	id, err := ids.New()
	if err != nil {
		return err
	}
	closed, err := repository.ActivateXNodePurchasePeriod(ctx, identity, previous, purchase, id, generation, s.now().UTC())
	if err != nil || closed.ID == "" || closed.Status == "closed" {
		return err
	}
	if err = s.revokePeriodProbes(ctx, closed.ID); err != nil {
		return err
	}
	return s.settlePeriodClosure(ctx, closed, true)
}

func (s *Service) purchasePeriodGeneration(ctx context.Context, identity Identity, expected int64) (int64, error) {
	if expected >= 0 {
		return expected, nil
	}
	user, err := s.repo.UserByID(ctx, identity.UserID)
	if err != nil {
		return 0, err
	}
	if user.ID != identity.UserID || user.AccountStatus != "active" || user.AccountGeneration < 0 {
		return 0, ErrConflict
	}
	return user.AccountGeneration, nil
}
