package xnode

import (
	"context"
	"slices"
)

// Authoritative entitlement checks deliberately ignore transport, enabled flags,
// readiness, quota exhaustion and short provisioning gaps. Those suspend runtime
// use without erasing a member's stable endpoint references.
func (s *Service) routingEntitlementEndpoints(ctx context.Context, user string) (routingInput, error) {
	in := routingInput{endpoints: map[string]Endpoint{}, byEndpoint: map[string]Endpoint{}}
	purchase, err := s.EffectivePurchase(ctx, user)
	if err != nil {
		return in, err
	}
	disabled, err := s.repo.UserDisabledSquads(ctx, user)
	if err != nil {
		return in, err
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return in, err
	}
	endpoints, err := s.Endpoints(ctx)
	if err != nil {
		return in, err
	}
	var accounts []PAYGAccount
	payg := map[string]bool{}
	if repo, ok := s.paygRepository(); ok {
		prefs, e := repo.XNodePAYGPreferences(ctx, user)
		if e != nil {
			return in, e
		}
		products, e := repo.ListSquadProductOverrides(ctx, false)
		if e != nil {
			return in, e
		}
		for _, product := range products {
			payg[product.GroupID] = product.BillingMode == "payg" && prefs.Allows(product.GroupID)
		}
		accounts, err = repo.XNodePAYGAccounts(ctx, user, "")
		if err != nil {
			return in, err
		}
	}
	for _, endpoint := range endpoints {
		var bound []string
		for _, group := range groups {
			for _, binding := range group.Bindings {
				if binding.NodeID == endpoint.NodeID && binding.InboundTag == endpoint.InboundTag {
					bound = append(bound, group.ID)
					break
				}
			}
		}
		bound, err = filterEndpointGroups(bound, endpoint.Overrides)
		if err != nil {
			return in, err
		}
		for _, group := range bound {
			if purchase != nil && slices.Contains(purchase.SquadUUIDs, group) && !slices.Contains(disabled, group) {
				in.byEndpoint[endpoint.ID] = endpoint
			}
			for _, account := range accounts {
				if account.GroupID == group && account.NodeID == endpoint.NodeID && !account.Disabled && payg[group] {
					copy := endpoint
					copy.ID = paygProxyID(endpoint.ID, group)
					in.byEndpoint[copy.ID] = copy
				}
			}
		}
	}
	for _, endpoint := range in.byEndpoint {
		old := in.endpoints[endpoint.NodeID]
		if old.ID == "" || endpoint.ID < old.ID {
			in.endpoints[endpoint.NodeID] = endpoint
		}
	}
	return in, nil
}
