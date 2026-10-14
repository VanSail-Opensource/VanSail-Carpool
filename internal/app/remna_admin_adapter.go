package app

import (
	"context"
	"fmt"
	"math"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
)

func (a remnaAdapter) ListInternalSquads(ctx context.Context) ([]admin.UpstreamSquad, error) {
	squads, err := a.listSquads(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]admin.UpstreamSquad, 0, len(squads))
	for _, squad := range squads {
		result = append(result, admin.UpstreamSquad{UUID: squad.UUID, Name: squad.Name})
	}
	return result, nil
}

func (a remnaAdapter) ListCatalogSquads(ctx context.Context) ([]catalog.RemoteSquad, error) {
	squads, err := a.listSquads(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]catalog.RemoteSquad, 0, len(squads))
	for _, squad := range squads {
		result = append(result, catalog.RemoteSquad{UUID: squad.UUID, Name: squad.Name})
	}
	return result, nil
}

func (a remnaAdapter) ListCatalogNodes(ctx context.Context) ([]catalog.RemoteNode, error) {
	nodes, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) ([]remnawave.Node, error) {
		return client.ListNodes(callCtx)
	})
	if err != nil {
		return nil, err
	}
	result := make([]catalog.RemoteNode, 0, len(nodes))
	for _, node := range nodes {
		a.multipliers.set(node.UUID, fixedNodeMultiplier(node.ConsumptionMultiplier))
		inbounds := make([]string, 0, len(node.ConfigProfile.ActiveInbounds))
		for _, inbound := range node.ConfigProfile.ActiveInbounds {
			inbounds = append(inbounds, inbound.UUID)
		}
		providerName := ""
		var providerFaviconURL *string
		if node.Provider != nil {
			providerName, providerFaviconURL = node.Provider.Name, node.Provider.FaviconLink
		}
		result = append(result, catalog.RemoteNode{UUID: node.UUID, Name: node.Name, CountryCode: node.CountryCode,
			ConsumptionMultiplier: node.ConsumptionMultiplier, ActiveInboundUUIDs: inbounds, Disabled: node.IsDisabled,
			ProviderName: providerName, ProviderFaviconURL: providerFaviconURL})
	}
	return result, nil
}

func fixedNodeMultiplier(value float64) int64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	const scale = 1_000_000
	return int64(math.Round(value * scale))
}

func (a remnaAdapter) nodeMultiplier(ctx context.Context, uuid string) (int64, error) {
	if value, ok := a.multipliers.get(uuid); ok {
		return value, nil
	}
	nodes, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) ([]remnawave.Node, error) {
		return client.ListNodes(callCtx)
	})
	if err != nil {
		return 0, err
	}
	for _, node := range nodes {
		a.multipliers.set(node.UUID, fixedNodeMultiplier(node.ConsumptionMultiplier))
	}
	if value, ok := a.multipliers.get(uuid); ok {
		return value, nil
	}
	return 0, fmt.Errorf("node multiplier is unavailable for %q", uuid)
}

func (a remnaAdapter) AccessibleCatalogNodeUUIDs(ctx context.Context, squadUUID string) ([]string, error) {
	nodes, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) ([]remnawave.AccessibleNode, error) {
		return client.InternalSquadAccessibleNodes(callCtx, squadUUID)
	})
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, node.UUID)
	}
	return result, nil
}

func (a remnaAdapter) listSquads(ctx context.Context) ([]remnawave.InternalSquad, error) {
	return remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) ([]remnawave.InternalSquad, error) {
		return client.ListInternalSquads(callCtx)
	})
}

var _ admin.SquadImporter = remnaAdapter{}
