package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

func (s *Service) resetServiceCadence(ctx context.Context, identity Identity, period Period, next time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.repo.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || current.CurrentPeriodID != period.ID {
		return err
	}
	access, err := s.ServiceAccess(ctx, current)
	if err != nil || access == nil {
		return err
	}
	if access.Disabled || !access.ExpiresAt.After(s.now()) {
		return nil
	}
	period, err = s.repo.XNodePeriod(ctx, current.CurrentPeriodID)
	if err != nil || !cadenceReady(period, s.now()) {
		return err
	}
	if allowed, err := s.cadenceRecoveryAllowed(ctx, current, period); err != nil || !allowed {
		return err
	}
	next = rollover.CadenceAdvance(period.StartedAt, period.Strategy)
	if err = s.closePeriod(ctx, period, true); err != nil {
		return err
	}
	for !rollover.CadenceAdvance(next, period.Strategy).After(s.now()) {
		next = rollover.CadenceAdvance(next, period.Strategy)
	}
	return s.startServicePeriod(ctx, current, *access, next)
}
