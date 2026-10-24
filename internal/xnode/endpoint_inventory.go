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

func (s *Service) comboInventory(ctx context.Context, userID string) ([]subscription.Proxy, bool, error) {
	access, err := s.memberAccess(ctx, userID)
	if err != nil || !access.allowed {
		return []subscription.Proxy{}, access.pending, err
	}
	identity, purchase := access.identity, access.purchase
	endpoints, err := s.Endpoints(ctx)
	if err != nil {
		return nil, false, err
	}
	disabled, err := s.repo.UserDisabledSquads(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	out := []subscription.Proxy{}
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
	return out, false, nil
}
