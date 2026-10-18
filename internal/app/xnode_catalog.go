package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
)

func (a proxyProvider) Dashboard(ctx context.Context, id string) (catalog.RemoteDashboard, error) {
	unlock := a.lockBridgeRead()
	defer unlock()
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return catalog.RemoteDashboard{}, err
	}
	if !native {
		return a.remnaAdapter.Dashboard(ctx, id)
	}
	// Display the committed local snapshot even during a source outage. Financial
	// settlement continues requiring successful reconciliation and sealed periods.
	_ = a.reconcileBridgeUsage(ctx, identity)
	statistics, link, err := a.native.Statistics(ctx, id)
	return catalog.RemoteDashboard{Statistics: statistics, SubscriptionURL: link}, err
}
func (a proxyProvider) RevokeSubscription(ctx context.Context, id string) (string, error) {
	_, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return "", err
	}
	if !native {
		return a.remnaAdapter.RevokeSubscription(ctx, id)
	}
	return a.native.RevokeSubscription(ctx, id)
}
func (a proxyProvider) ListInternalSquads(ctx context.Context) ([]admin.UpstreamSquad, error) {
	groups, err := a.native.Groups(ctx)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		legacy, err := a.legacy(ctx)
		if err != nil {
			return nil, err
		}
		if legacy {
			return a.remnaAdapter.ListInternalSquads(ctx)
		}
	}
	out := []admin.UpstreamSquad{}
	for _, group := range groups {
		out = append(out, admin.UpstreamSquad{UUID: group.ID, Name: group.Name})
	}
	return out, nil
}
func (a proxyProvider) ListCatalogSquads(ctx context.Context) ([]catalog.RemoteSquad, error) {
	groups, err := a.native.Groups(ctx)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		bridge, err := a.legacy(ctx)
		if err != nil {
			return nil, err
		}
		if bridge {
			return a.remnaAdapter.ListCatalogSquads(ctx)
		}
	}
	out := []catalog.RemoteSquad{}
	for _, group := range groups {
		if group.Enabled {
			out = append(out, catalog.RemoteSquad{UUID: group.ID, Name: group.Name})
		}
	}
	return out, nil
}
func (a proxyProvider) ListCatalogNodes(ctx context.Context) ([]catalog.RemoteNode, error) {
	nodes, err := a.native.Nodes(ctx)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		legacy, err := a.legacy(ctx)
		if err != nil {
			return nil, err
		}
		if legacy {
			return a.remnaAdapter.ListCatalogNodes(ctx)
		}
	}
	groups, err := a.native.Groups(ctx)
	if err != nil {
		return nil, err
	}
	out := []catalog.RemoteNode{}
	for _, node := range nodes {
		tags := []string{}
		seen := map[string]bool{}
		for _, group := range groups {
			if !group.Enabled {
				continue
			}
			for _, binding := range group.Bindings {
				if binding.NodeID == node.ID && !seen[binding.InboundTag] {
					tags = append(tags, proxyInboundID(node.ID, binding.InboundTag))
					seen[binding.InboundTag] = true
				}
			}
		}
		out = append(out, catalog.RemoteNode{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode, ConsumptionMultiplier: node.Multiplier, ActiveInboundUUIDs: tags, Disabled: !node.Enabled, ProviderName: node.ProviderName})
	}
	return out, nil
}
func (a proxyProvider) AccessibleCatalogNodeUUIDs(ctx context.Context, id string) ([]string, error) {
	groups, err := a.native.Groups(ctx)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		legacy, err := a.legacy(ctx)
		if err != nil {
			return nil, err
		}
		if legacy {
			return a.remnaAdapter.AccessibleCatalogNodeUUIDs(ctx, id)
		}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, group := range groups {
		if group.ID != id || !group.Enabled {
			continue
		}
		for _, binding := range group.Bindings {
			if !seen[binding.NodeID] {
				out = append(out, binding.NodeID)
				seen[binding.NodeID] = true
			}
		}
	}
	return out, nil
}
func (a proxyProvider) SquadNames(ctx context.Context) (map[string]string, error) {
	groups, err := a.ListInternalSquads(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, group := range groups {
		out[group.UUID] = group.Name
	}
	return out, nil
}
func (a proxyProvider) MemberSquadNames(ctx context.Context) (map[string]string, error) {
	return a.SquadNames(ctx)
}
