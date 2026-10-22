package xnode

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// memberAccess is shared by exports and agent authorization. An unprepared paid
// period is pending provisioning, never an expired subscription or a fresh quota.
type memberAccess struct {
	identity Identity
	purchase *model.Purchase
	period   *Period
	allowed  bool
	pending  bool
}

func (s *Service) memberAccess(ctx context.Context, id string) (memberAccess, error) {
	var access memberAccess
	user, err := s.repo.UserByID(ctx, id)
	if err != nil || user.OnboardingState != "complete" {
		return access, err
	}
	access.purchase, err = s.EffectivePurchase(ctx, id)
	if err != nil || access.purchase == nil {
		return access, err
	}
	restricted, err := s.repo.XNodeUserRestricted(ctx, id)
	if err != nil || restricted {
		return access, err
	}
	access.identity, err = s.repo.XNodeIdentityByUser(ctx, id)
	if isMissing(err) {
		access.pending = true
		return access, nil
	}
	if err != nil || access.identity.AccessDisabled {
		return access, err
	}
	if access.identity.CurrentPeriodID == "" {
		access.pending = true
		return access, nil
	}
	period, err := s.repo.XNodePeriod(ctx, access.identity.CurrentPeriodID)
	if isMissing(err) {
		access.pending = true
		return access, nil
	}
	if err != nil {
		return access, err
	}
	access.period = &period
	if period.PurchaseID != access.purchase.ID || period.Status != "open" {
		access.pending = true
		return access, nil
	}
	access.allowed = period.LimitBytes == 0 || period.UsedBytes < period.LimitBytes
	return access, nil
}
