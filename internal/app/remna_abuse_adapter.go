package app

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func (a remnaAdapter) AbuseNodes(ctx context.Context) ([]abuse.Node, error) {
	nodes, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) ([]abuse.Node, error) {
		items, callErr := client.ListNodes(callCtx)
		if callErr != nil {
			return nil, callErr
		}
		out := make([]abuse.Node, 0, len(items))
		for _, item := range items {
			out = append(out, abuse.Node{UUID: item.UUID, Name: item.Name})
		}
		return out, nil
	})
	return nodes, err
}

var _ abuse.NodeProvider = remnaAdapter{}
