package xnode

import (
	"context"
	"slices"
)

// Metrics retain the paid node scope when quota or account policy blocks access,
// so a denied member still sees an accurate reason instead of an empty zero.
func (s *Service) metricNodeScope(ctx context.Context, identity Identity) (map[string]bool, []UserLimit, error) {
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
	payg, err := s.paygInventory(ctx, identity.UserID)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range payg {
		nodes[p.NodeID] = true
	}
	if purchase == nil && len(payg) == 0 {
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
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, group := range groups {
		if !group.Enabled || !slices.Contains(purchase.SquadUUIDs, group.ID) || slices.Contains(disabled, group.ID) {
			continue
		}
		for _, binding := range group.Bindings {
			node, err := s.repo.XNodeNode(ctx, binding.NodeID)
			if err != nil {
				return nil, nil, err
			}
			if !node.Enabled {
				continue
			}
			owned, err := s.NativeOwner(ctx, node.ID)
			if err != nil {
				return nil, nil, err
			}
			nodes[node.ID] = owned
		}
	}
	return nodes, reasons, nil
}
