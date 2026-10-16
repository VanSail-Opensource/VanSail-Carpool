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
	if err = s.closePeriod(ctx, period, true); err != nil {
		return err
	}
	for !rollover.CadenceAdvance(next, period.Strategy).After(s.now()) {
		next = rollover.CadenceAdvance(next, period.Strategy)
	}
	return s.startServicePeriod(ctx, current, *access, next)
}
