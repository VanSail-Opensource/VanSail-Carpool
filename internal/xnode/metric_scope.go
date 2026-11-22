package xnode

import (
	"context"
	"slices"
)

// Metrics retain the paid node scope when quota or account policy blocks access,
// so a denied member still sees an accurate reason instead of an empty zero.
func (s *Service) metricNodeScope(ctx context.Context, identity Identity) (map[string]bool, []UserLimit, error) {
	projection := newInventoryProjection(s)
	nodes := map[string]bool{}
	reasons := []UserLimit{}
	purchase, err := s.EffectivePurchase(ctx, identity.UserID)
	if err != nil {
		return nil, nil, err
	}
	restricted, err := s.repo.XNodeUserRestricted(ctx, identity.UserID)
	if err != nil {
		return nil, nil, err
	}
	if restricted {
		reasons = append(reasons, UserLimit{Reason: "account_disabled", Scope: "all_connections"})
	}
	payg, err := s.paygInventory(ctx, identity.UserID, projection)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range payg {
		nodes[p.NodeID] = true
	}
	hourly, err := s.hourlyInventory(ctx, identity.UserID, projection)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range hourly {
		nodes[p.NodeID] = true
	}
	paidHourly, err := s.hasPaidHourlyBenefit(ctx, identity.UserID)
	if err != nil {
		return nil, nil, err
	}
	if purchase == nil && len(payg) == 0 && !paidHourly {
		reasons = append(reasons, UserLimit{Reason: "authorization_expired", Scope: "all_connections"})
	}
	if identity.CurrentPeriodID != "" {
		period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil {
			return nil, nil, err
		}
		if period.LimitBytes > 0 && period.UsedBytes >= period.LimitBytes {
			reasons = append(reasons, UserLimit{Reason: "quota_exceeded", Scope: "all_connections"})
		}
		if purchase == nil {
			purchases, err := s.repo.ListPurchases(ctx, identity.UserID)
			if err != nil {
				return nil, nil, err
			}
			for _, item := range purchases {
				if item.ID == period.PurchaseID {
					purchase = &item
					break
				}
			}
		}
	}
	if purchase == nil {
		return nodes, reasons, nil
	}
	disabled, err := s.repo.UserDisabledSquads(ctx, identity.UserID)
	if err != nil {
		return nil, nil, err
	}
	groups, err := projection.readGroups(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, group := range groups {
		if !group.Enabled || !slices.Contains(purchase.SquadUUIDs, group.ID) || slices.Contains(disabled, group.ID) {
			continue
		}
		for _, binding := range group.Bindings {
			node, err := projection.readNode(ctx, binding.NodeID)
			if err != nil {
				return nil, nil, err
			}
			if !node.Enabled {
				continue
			}
			nodes[node.ID] = node.RetiredAt == nil
		}
	}
	return nodes, reasons, nil
}
