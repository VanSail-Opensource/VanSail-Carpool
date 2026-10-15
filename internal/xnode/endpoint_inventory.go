package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"slices"
)

func (s *Service) MemberProxies(ctx context.Context, userID string) ([]subscription.Proxy, error) {
	proxies, _, err := s.memberInventory(ctx, userID)
	return proxies, err
}

func (s *Service) memberInventory(ctx context.Context, userID string) ([]subscription.Proxy, bool, error) {
	identity, err := s.repo.XNodeIdentityByUser(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	purchase, err := s.EffectivePurchase(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	if purchase == nil {
		return []subscription.Proxy{}, false, nil
	}
	restricted, err := s.repo.XNodeUserRestricted(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	if restricted {
		return []subscription.Proxy{}, false, nil
	}
	if identity.CurrentPeriodID != "" {
		period, err := s.repo.XNodePeriod(ctx, identity.CurrentPeriodID)
		if err != nil {
			return nil, false, err
		}
		if period.Status != "open" || period.LimitBytes > 0 && period.UsedBytes >= period.LimitBytes {
			return []subscription.Proxy{}, false, nil
		}
	}
	endpoints, err := s.Endpoints(ctx)
	if err != nil {
		return nil, false, err
	}
	disabled, err := s.repo.UserDisabledSquads(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	out := []subscription.Proxy{}
	if s.legacyProxies != nil && identity.LegacyUserID != "" {
		out, err = s.legacyProxies(ctx, identity)
		if err != nil {
			out = []subscription.Proxy{}
		}
	}
	legacyError := err
	for _, endpoint := range endpoints {
		if !endpoint.Enabled {
			continue
		}
		node, err := s.repo.XNodeNode(ctx, endpoint.NodeID)
		if err != nil {
			return nil, false, err
		}
		if !node.Enabled {
			continue
		}
		owned, err := s.NativeOwner(ctx, node.ID)
		if err != nil {
			return nil, false, err
		}
		if !owned {
			continue
		}
		tags, _, err := s.eligibleTags(ctx, userID, node.ID)
		if err != nil {
			return nil, false, err
		}
		if !slices.Contains(tags, endpoint.InboundTag) {
			continue
		}
		proxy, err := s.endpointProxy(ctx, identity, endpoint)
		if err != nil {
			return nil, false, err
		}
		proxy.GroupIDs = slices.DeleteFunc(proxy.GroupIDs, func(group string) bool {
			return !slices.Contains(purchase.SquadUUIDs, group) || slices.Contains(disabled, group)
		})
		proxy.GroupIDs, err = filterEndpointGroups(proxy.GroupIDs, endpoint.Overrides)
		if err != nil {
			return nil, false, err
		}
		if len(proxy.GroupIDs) == 0 {
			continue
		}
		out = append(out, proxy)
	}
	if len(out) == 0 && legacyError != nil {
		return nil, true, ErrUnavailable
	}
	return out, legacyError != nil, nil
}
