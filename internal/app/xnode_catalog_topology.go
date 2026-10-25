package app

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
)

// One authoritative shared snapshot replaces per-squad full group-list reads.
func (a proxyProvider) ListCatalogTopology(ctx context.Context) (catalog.RemoteTopology, error) {
	source, err := a.native.CatalogTopology(ctx)
	if err != nil {
		return catalog.RemoteTopology{}, err
	}
	out := catalog.RemoteTopology{Squads: []catalog.RemoteSquad{}, Nodes: []catalog.RemoteNode{}, Accessible: map[string][]string{}}
	tags := map[string][]string{}
	seenTags := map[string]bool{}
	for _, group := range source.Groups {
		if !group.Enabled {
			continue
		}
		out.Squads = append(out.Squads, catalog.RemoteSquad{UUID: group.ID, Name: group.Name})
		seen := map[string]bool{}
		for _, binding := range group.Bindings {
			tag := proxyInboundID(binding.NodeID, binding.InboundTag)
			if !seenTags[tag] {
				tags[binding.NodeID] = append(tags[binding.NodeID], tag)
				seenTags[tag] = true
			}
			if !seen[binding.NodeID] {
				out.Accessible[group.ID] = append(out.Accessible[group.ID], binding.NodeID)
				seen[binding.NodeID] = true
			}
		}
	}
	for _, node := range source.Nodes {
		out.Nodes = append(out.Nodes, catalog.RemoteNode{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode,
			ConsumptionMultiplier: node.Multiplier, Disabled: !node.Enabled, ProviderName: node.ProviderName, ActiveInboundUUIDs: tags[node.ID]})
	}
	return out, nil
}
