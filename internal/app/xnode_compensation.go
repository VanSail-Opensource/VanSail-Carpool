package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
	"github.com/google/uuid"
)

func (a proxyProvider) CompensationNodes(ctx context.Context) ([]compensation.Node, error) {
	if nodes, err := a.native.Nodes(ctx); err != nil {
		return nil, err
	} else if len(nodes) == 0 {
		return a.remnaAdapter.CompensationNodes(ctx)
	}
	nodes, err := a.ListCatalogNodes(ctx)
	if err != nil {
		return nil, err
	}
	out := []compensation.Node{}
	for _, node := range nodes {
		out = append(out, compensation.Node{UUID: node.UUID, Name: node.Name, Disabled: node.Disabled, Connected: a.master.Connected(node.UUID), ActiveInboundUUIDs: node.ActiveInboundUUIDs})
	}
	bridge, err := a.legacy(ctx)
	if err != nil {
		return nil, err
	}
	if bridge {
		legacy, err := a.remnaAdapter.CompensationNodes(ctx)
		if err != nil {
			return nil, err
		}
		for _, remote := range legacy {
			owned, err := a.native.NativeOwner(ctx, remote.UUID)
			if err != nil {
				return nil, err
			}
			if owned {
				continue
			}
			for i := range out {
				if out[i].UUID == remote.UUID {
					out[i].Connected = remote.Connected
					break
				}
			}
		}
	}
	return out, nil
}
func (a proxyProvider) CompensationSquads(ctx context.Context) ([]compensation.Squad, error) {
	groups, err := a.native.Groups(ctx)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return a.remnaAdapter.CompensationSquads(ctx)
	}
	out := []compensation.Squad{}
	for _, group := range groups {
		item := compensation.Squad{UUID: group.ID, Name: group.Name}
		for _, binding := range group.Bindings {
			item.Inbounds = append(item.Inbounds, proxyInboundID(binding.NodeID, binding.InboundTag))
		}
		out = append(out, item)
	}
	return out, nil
}

func proxyInboundID(node, tag string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("vansail:inbound:"+node+":"+tag)).String()
}
