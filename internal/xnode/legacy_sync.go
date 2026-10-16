package xnode

import (
	"context"
	"time"
)

// SyncLegacyUsage retains Remnawave's already weighted quota/lifetime counters.
// Reset timestamps select a new native accounting period rather than moving late
// native reports into that period. Compatible credentials remain unchanged.
func (s *Service) SyncLegacyUsage(ctx context.Context, identity Identity, next LegacyUsage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.repo.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil {
		return err
	}
	if current.UserID == "" {
		return s.syncServiceUsage(ctx, current, next)
	}
	purchase, err := s.EffectivePurchase(ctx, identity.UserID)
	if err != nil {
		return err
	}
	previous, previousErr := s.repo.XNodeLegacyUsage(ctx, identity.ProxyUserID)
	if previousErr != nil && !isMissing(previousErr) {
		return previousErr
	}
	// A preview imported after the background bridge started must not rotate or
	// reprice a newer snapshot. Ignore stale exports before changing any periods.
	if previousErr == nil && !next.SyncedAt.After(previous.SyncedAt) {
		return nil
	}
	if purchase != nil && current.CurrentPeriodID == "" {
		at := purchase.ValidFrom
		if next.LastResetAt != nil && next.LastResetAt.After(at) {
			at = *next.LastResetAt
		}
		if err = s.startPeriod(ctx, current, *purchase, at); err != nil {
			return err
		}
		current, err = s.repo.XNodeIdentity(ctx, current.ProxyUserID)
		if err != nil {
			return err
		}
	}
	if current.CurrentPeriodID != "" && previousErr == nil && previous.PeriodID == current.CurrentPeriodID && changedLegacyReset(previous.LastResetAt, next.LastResetAt) && purchase != nil {
		period, err := s.repo.XNodePeriod(ctx, current.CurrentPeriodID)
		if err != nil {
			return err
		}
		if period.Status == "open" || period.Status == "closing" {
			if err = s.closePeriod(ctx, period, true); err != nil {
				return err
			}
			at := s.now().UTC()
			if next.LastResetAt != nil {
				at = *next.LastResetAt
			}
			if at.Before(purchase.ValidFrom) {
				at = purchase.ValidFrom
			}
			if err = s.startPeriod(ctx, current, *purchase, at); err != nil {
				return err
			}
			current, err = s.repo.XNodeIdentity(ctx, current.ProxyUserID)
			if err != nil {
				return err
			}
		}
	}
	next.ProxyUserID = current.ProxyUserID
	next.PeriodID = current.CurrentPeriodID
	return s.repo.SyncXNodeLegacyUsage(ctx, next)
}
func changedLegacyReset(before, after *time.Time) bool {
	if before == nil || after == nil {
		return before != after
	}
	return !before.Equal(*after)
}
