package xnode

import (
	"context"
)

// routingPurchaseRenewing reports whether the effective purchase is at a
// renewal boundary rather than durably absent: a paid successor is waiting, an
// activation is in flight, or an enabled automatic renewal may still settle its
// successor. ProcessDueAutoRenewals may defer settlement while pricing, stock or
// health evidence is unresolved; the failure path records the failure and
// clears auto_renew_enabled, so an enabled preference is durable renewal
// evidence with no time bound. The endpoint inventory derived from
// EffectivePurchase is incomplete during those windows, so it must not be
// treated as durable access loss.
func (s *Service) routingPurchaseRenewing(ctx context.Context, user string) (bool, error) {
	current, err := s.EffectivePurchase(ctx, user)
	if err != nil || current != nil {
		return false, err
	}
	purchases, err := s.repo.ListPurchases(ctx, user)
	if err != nil {
		return false, err
	}
	now := s.now()
	for _, purchase := range purchases {
		switch purchase.Status {
		case "queued", "activating":
			// The successor is durable and paid or already activating.
			if purchase.ValidUntil.After(now) {
				return true, nil
			}
		case "active":
			// DesiredEntitlement already excludes an expired term. Auto-renewal
			// settles at or after valid_until and may be deferred; keep the plan
			// while the durable preference is enabled. Retained rules do not grant
			// access: compilation still requires an effective entitlement.
			if purchase.AutoRenewEnabled && !purchase.ValidUntil.After(now) {
				return true, nil
			}
		}
	}
	return false, nil
}
