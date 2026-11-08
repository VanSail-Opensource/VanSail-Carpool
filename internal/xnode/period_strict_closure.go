package xnode

import "context"

type identityClosure struct {
	identity Identity
	period   Period
	purchase string
}

// captureIdentityClosure fences admission before dropping the global mutation
// lock. A later paid successor remains a different, untouched period.
func (s *Service) captureIdentityClosure(ctx context.Context, id, purchase string, onlyClosing bool) (identityClosure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return identityClosure{}, err
	}
	return s.captureIdentityClosureLocked(ctx, identity, purchase, onlyClosing)
}

func (s *Service) captureIdentityClosureLocked(ctx context.Context, identity Identity, purchase string, onlyClosing bool) (identityClosure, error) {
	target := identityClosure{identity: identity, purchase: purchase}
	if identity.CurrentPeriodID == "" {
		if purchase != "" {
			return target, ErrConflict
		}
		return target, nil
	}
	period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		return target, err
	}
	if period.ProxyUserID != identity.ProxyUserID {
		return target, ErrConflict
	}
	if err = s.checkPurchaseOperation(ctx, identity, period, purchase); err != nil {
		return target, err
	}
	if !onlyClosing || period.Status == "closing" {
		period, err = s.preparePeriodClosure(ctx, period)
	}
	target.period = period
	return target, err
}

func (s *Service) finishIdentityClosure(ctx context.Context, target identityClosure) (Period, error) {
	if target.period.ID != "" {
		if err := s.settlePeriodClosure(ctx, target.period, false); err != nil {
			return Period{}, err
		}
	}
	return s.checkIdentityClosure(ctx, target)
}

// checkIdentityClosure validates the original snapshot without native calls.
// Refund aggregation must not substitute a newly captured closure generation.
func (s *Service) checkIdentityClosure(ctx context.Context, target identityClosure) (Period, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.repo.XNodeIdentity(ctx, target.identity.ProxyUserID)
	if err != nil {
		return Period{}, err
	}
	if identity.ProxyUserID != target.identity.ProxyUserID || identity.UserID != target.identity.UserID || identity.MemberOwnerID() != target.identity.MemberOwnerID() || identity.CurrentPeriodID != target.identity.CurrentPeriodID {
		return Period{}, ErrConflict
	}
	if target.period.ID == "" {
		return Period{}, nil
	}
	period, err := s.repo.XNodePeriod(ctx, target.period.ID)
	if err != nil {
		return Period{}, err
	}
	if !samePeriodClosure(period, target.period) || period.Status != "closed" {
		return Period{}, ErrConflict
	}
	if err = s.checkPurchaseOperation(ctx, identity, period, target.purchase); err != nil {
		return Period{}, err
	}
	return period, nil
}

func (s *Service) quiesceIdentity(ctx context.Context, id, purchase string) error {
	target, err := s.captureIdentityClosure(ctx, id, purchase, false)
	if err != nil {
		return err
	}
	_, err = s.finishIdentityClosure(ctx, target)
	return err
}
