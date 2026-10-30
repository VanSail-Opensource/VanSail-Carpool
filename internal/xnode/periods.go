package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"time"
)

// ApplyPurchase is idempotent for an existing term. A replacement closes the old
// period only after all participating agents' traffic journals are acknowledged.
func (s *Service) ApplyPurchase(ctx context.Context, id string, limit int64, strategy string, expires time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return err
	}
	purchase, err := s.requestedPurchase(ctx, identity.UserID, limit, strategy, expires)
	if err != nil {
		return err
	}
	if purchase == nil {
		return s.closeIdentityPeriod(ctx, identity)
	}
	if limit != purchase.TrafficLimitBytes || strategy != purchase.ResetStrategy || !expires.Equal(purchase.ValidUntil) {
		return ErrConflict
	}
	if identity.CurrentPeriodID != "" {
		period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil {
			return err
		}
		if period.PurchaseID == purchase.ID && period.Status == "open" {
			if err := s.normalizePurchaseSquads(ctx, identity, *purchase); err != nil {
				return err
			}
			period.LimitBytes = limit
			period.Strategy = strategy
			return s.repo.SaveXNodePeriod(ctx, period)
		}
		if err = s.closePeriod(ctx, period, true); err != nil {
			return err
		}
	}
	return s.startPeriod(ctx, identity, *purchase, s.now().UTC())
}
func (s *Service) startPeriod(ctx context.Context, identity Identity, purchase model.Purchase, at time.Time, reference ...string) error {
	if err := s.normalizePurchaseSquads(ctx, identity, purchase); err != nil {
		return err
	}
	var id string
	if len(reference) > 0 {
		id = reference[0]
	} else {
		var err error
		id, err = ids.New()
		if err != nil {
			return err
		}
	}
	return s.repo.SaveXNodePeriod(ctx, Period{ID: id, ProxyUserID: identity.ProxyUserID, PurchaseID: purchase.ID, StartedAt: at, LimitBytes: purchase.TrafficLimitBytes, Strategy: purchase.ResetStrategy, LastResetAt: &at, Status: "open"})
}

func (s *Service) normalizePurchaseSquads(ctx context.Context, identity Identity, purchase model.Purchase) error {
	if identity.UserID == "" {
		return nil
	}
	if preferences, ok := s.repo.(interface {
		EnabledSquadsForUser(context.Context, string, []string) ([]string, error)
	}); ok {
		_, err := preferences.EnabledSquadsForUser(ctx, identity.UserID, purchase.SquadUUIDs)
		return err
	}
	return nil
}
func (s *Service) closeIdentityPeriod(ctx context.Context, identity Identity) error {
	if identity.CurrentPeriodID == "" {
		return nil
	}
	period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		return err
	}
	return s.closePeriod(ctx, period)
}
func (s *Service) closePeriod(ctx context.Context, period Period, allowPending ...bool) error {
	canContinue := len(allowPending) > 0 && allowPending[0]
	if period.Status == "closed" {
		return nil
	}
	if period.Status == "review" {
		return ErrConflict
	}
	if period.Status == "open" {
		period.Status = "closing"
		at := s.now().UTC()
		period.EndedAt = &at
		if err := s.repo.SaveXNodePeriod(ctx, period); err != nil {
			return err
		}
	}
	// Synchronize before reading participants: earlier in-flight grants must finish
	// and record their participation before revocation overtakes them.
	if err := s.revokePeriodProbes(ctx, period.ID); err != nil {
		return err
	}
	nodes, err := s.repo.XNodeNodes(ctx)
	if err != nil {
		return err
	}
	if canContinue && s.master != nil {
		for _, node := range nodes {
			if s.master.Connected(node.ID) {
				// Revocation is durable. The existing reconciler publishes it,
				// and ResumeAccounting settles the old journal under a deadline.
				// A paid successor must not wait behind an online replay backlog
				// while the caller holds the shared service mutation mutex.
				select {
				case s.master.stateWake <- struct{}{}:
				default:
				}
				return nil
			}
		}
	}
	pending := false
	for _, node := range nodes {
		if s.master != nil && s.master.Connected(node.ID) {
			if err = s.master.Sync(ctx, node.ID); err != nil {
				if canContinue {
					pending = true
					continue
				}
				return err
			}
		}
	}
	participants, err := s.repo.XNodePeriodNodes(ctx, period.ID)
	if err != nil {
		return err
	}
	for _, node := range participants {
		if node.Flushed {
			continue
		}
		if s.master == nil || !s.master.Connected(node.NodeID) {
			if !canContinue {
				return ErrUnavailable
			}
			pending = true
			continue
		}
		if err = s.flushPeriodProbes(ctx, period.ID, node.NodeID); err != nil {
			if canContinue {
				pending = true
				continue
			}
			return err
		}
		barrier, err := s.master.FlushPeriod(ctx, node.NodeID, period.ProxyUserID, period.ID)
		if err != nil {
			if canContinue {
				pending = true
				continue
			}
			return err
		}
		if err = s.repo.SealXNodePeriodNode(ctx, period.ID, node.NodeID, barrier.Sequence); err != nil {
			return err
		}
	}
	if pending {
		if !canContinue {
			return ErrUnavailable
		}
		return nil
	}
	period.Status = "closed"
	return s.repo.SaveXNodePeriod(ctx, period)
}
