package xnode

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
	"time"
)

// ResetCadences advances paid quota periods without charging for scheduled resets.
func (s *Service) ResetCadences(ctx context.Context) error {
	identities, err := s.cadenceIdentities(ctx)
	if err != nil {
		return err
	}
	var failures error
	for _, identity := range identities {
		if ctx.Err() != nil {
			return errors.Join(failures, ctx.Err())
		}
		failures = errors.Join(failures, s.resetCadence(ctx, identity))
	}
	return failures
}
func (s *Service) resetCadence(ctx context.Context, identity Identity) error {
	if identity.CurrentPeriodID == "" {
		return nil
	}
	period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		return err
	}
	if !cadenceReady(period, s.now()) {
		return nil
	}
	next := rollover.CadenceAdvance(period.StartedAt, period.Strategy)
	if next.After(s.now()) {
		return nil
	}
	if identity.UserID == "" {
		return s.resetServiceCadence(ctx, identity, period, next)
	}
	purchase, err := s.EffectivePurchase(ctx, identity.UserID)
	if err != nil {
		return err
	}
	if purchase == nil || purchase.ID != period.PurchaseID {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.repo.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || current.CurrentPeriodID != period.ID {
		return err
	}
	period, err = s.repo.XNodePeriod(ctx, current.CurrentPeriodID)
	if err != nil || !cadenceReady(period, s.now()) {
		return err
	}
	purchase, err = s.EffectivePurchase(ctx, identity.UserID)
	if err != nil || purchase == nil || purchase.ID != period.PurchaseID {
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
	return s.startPeriod(ctx, identity, *purchase, next)
}

// ResumeAccounting seals revoked periods after reconnects without quiescing the
// successor's streams. A missing journal leaves only that settlement pending.
func (s *Service) ResumeAccounting(ctx context.Context) error {
	periods, err := s.repo.XNodeClosingPeriods(ctx, 16)
	if err != nil {
		return err
	}
	var failures error
	for _, period := range periods {
		if ctx.Err() != nil {
			return errors.Join(failures, ctx.Err())
		}
		child, cancel := context.WithTimeout(ctx, 2*time.Second)
		s.mu.Lock()
		current, err := s.repo.XNodePeriod(child, period.ID)
		if err == nil && current.Status == "closing" {
			err = s.closePeriod(child, current)
		}
		s.mu.Unlock()
		cancel()
		failures = errors.Join(failures, err)
	}
	return failures
}
func (s *Service) RunAccounting(ctx context.Context, onError func(error)) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			child, cancel := context.WithTimeout(ctx, 45*time.Second)
			err := s.ResetCadences(child)
			err = errors.Join(err, s.ResumeAccounting(child))
			cancel()
			if err != nil && ctx.Err() == nil {
				onError(err)
			}
		}
	}
}
