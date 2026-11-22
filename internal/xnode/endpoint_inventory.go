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

func (s *Service) comboInventory(ctx context.Context, userID string, projection *inventoryProjection) ([]subscription.Proxy, bool, error) {
	access, err := s.memberAccess(ctx, userID)
	if err != nil || !access.allowed {
		return []subscription.Proxy{}, access.pending, err
	}
	identity, purchase := access.identity, access.purchase
	endpoints, err := projection.readEndpoints(ctx)
	if err != nil {
		return nil, false, err
	}
	disabled, err := s.repo.UserDisabledSquads(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	groups, err := projection.readGroups(ctx)
	if err != nil {
		return nil, false, err
	}
	allowed := map[string]bool{}
	for _, group := range groups {
		allowed[group.ID] = group.Enabled && slices.Contains(purchase.SquadUUIDs, group.ID) && !slices.Contains(disabled, group.ID)
	}
	out := []subscription.Proxy{}
	for _, endpoint := range endpoints {
		if !endpoint.Enabled || !slices.ContainsFunc(projection.endpointGroups(endpoint), func(group string) bool { return allowed[group] }) {
			continue
		}
		node, err := projection.readNode(ctx, endpoint.NodeID)
		if err != nil {
			return nil, false, err
		}
		if !node.Enabled || node.RetiredAt != nil {
			continue
		}
		proxy, err := projection.proxy(ctx, identity, endpoint)
		if unusableEndpoint(err) {
			continue
		}
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
