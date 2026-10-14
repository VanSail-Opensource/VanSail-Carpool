package app

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
)

func (a remnaAdapter) CompensationNodes(ctx context.Context) ([]compensation.Node, error) {
	nodes, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) ([]remnawave.Node, error) {
		return client.ListNodes(callCtx)
	})
	if err != nil {
		return nil, err
	}
	result := make([]compensation.Node, 0, len(nodes))
	for _, node := range nodes {
		item := compensation.Node{UUID: node.UUID, Name: node.Name, Connected: node.IsConnected, Disabled: node.IsDisabled}
		for _, inbound := range node.ConfigProfile.ActiveInbounds {
			item.ActiveInboundUUIDs = append(item.ActiveInboundUUIDs, inbound.UUID)
		}
		result = append(result, item)
	}
	return result, nil
}

func (a remnaAdapter) CompensationSquads(ctx context.Context) ([]compensation.Squad, error) {
	squads, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) ([]remnawave.InternalSquad, error) {
		return client.ListInternalSquads(callCtx)
	})
	if err != nil {
		return nil, err
	}
	result := make([]compensation.Squad, 0, len(squads))
	for _, squad := range squads {
		item := compensation.Squad{UUID: squad.UUID, Name: squad.Name}
		for _, inbound := range squad.Inbounds {
			item.Inbounds = append(item.Inbounds, inbound.UUID)
		}
		result = append(result, item)
	}
	return result, nil
}

var _ compensation.Provider = remnaAdapter{}
