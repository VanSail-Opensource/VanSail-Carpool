package xnode

import (
	"context"
	"fmt"
	"sort"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
)

func (s *Service) loadRoutingEndpoints(ctx context.Context, identity Identity, in *routingInput) error {
	in.byEndpoint, in.sources, in.targets = map[string]Endpoint{}, map[string]Endpoint{}, map[string]Endpoint{}
	in.endpointInbounds, in.vertices = map[string]map[string]any{}, map[string]string{}
	in.available = []RoutingEndpoint{}
	proxies, err := s.MemberProxies(ctx, identity.UserID)
	if err != nil {
		return fmt.Errorf("routing endpoint inventory: %w", err)
	}
	allowed := map[string]bool{}
	for _, proxy := range proxies {
		allowed[proxy.ID] = true
	}
	endpoints, err := s.Endpoints(ctx)
	if err != nil {
		return err
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].ID < endpoints[j].ID })
	for _, endpoint := range endpoints {
		if !endpoint.Enabled || !allowed[endpoint.ID] {
			continue
		}
		owned, err := s.NativeOwner(ctx, endpoint.NodeID)
		if err != nil {
			return err
		}
		if !owned {
			continue
		}
		in.byEndpoint[endpoint.ID] = endpoint
		in.vertices[routingVertex(endpoint)] = endpoint.NodeID
		in.available = append(in.available, RoutingEndpoint{ID: endpoint.ID, NodeID: endpoint.NodeID, Name: endpoint.Name, Address: endpoint.Address, Port: endpoint.Port})
		if in.endpoints[endpoint.NodeID].ID == "" {
			in.endpoints[endpoint.NodeID] = endpoint // Preserve the previous deterministic default.
		}
	}
	for id := range in.endpoints {
		node, err := s.repo.XNodeNode(ctx, id)
		if err != nil {
			return err
		}
		in.nodes = append(in.nodes, RoutingNode{ID: id, Name: node.Name})
	}
	sort.Slice(in.nodes, func(i, j int) bool { return in.nodes[i].Name < in.nodes[j].Name })
	return nil
}

func (in *routingInput) resolveChoices(choices []RoutingChoice) error {
	in.legacy = true
	for _, choice := range choices {
		if !memberTagPattern.MatchString(choice.ID) || choice.InboundNodeID == choice.OutboundNodeID {
			return ErrInvalid
		}
		in.legacy = in.legacy && choice.InboundEndpointID == "" && choice.OutboundEndpointID == ""
		source, target := in.byEndpoint[choice.InboundEndpointID], in.byEndpoint[choice.OutboundEndpointID]
		if choice.InboundEndpointID == "" {
			source = in.endpoints[choice.InboundNodeID]
		}
		if choice.OutboundEndpointID == "" {
			target = in.endpoints[choice.OutboundNodeID]
		}
		if source.ID == "" || target.ID == "" || source.NodeID != choice.InboundNodeID || target.NodeID != choice.OutboundNodeID {
			return fmt.Errorf("%w: %w", ErrInvalid, &routingmatch.Diagnostic{Code: "routing_endpoint_unavailable", Rules: []string{choice.ID}})
		}
		choice.InboundEndpointID, choice.OutboundEndpointID = source.ID, target.ID
		in.choices = append(in.choices, choice)
		in.sources[choice.ID], in.targets[choice.ID] = source, target
	}
	return nil
}

func routingVertex(endpoint Endpoint) string { return endpoint.NodeID + "\x1f" + endpoint.InboundTag }

func (in routingInput) publicPaths(paths []routingmatch.Path) []routingmatch.Path {
	paths = routingPublicPaths(paths)
	for index := range paths {
		for hop, vertex := range paths[index].Nodes {
			if node := in.vertices[vertex]; node != "" {
				paths[index].Nodes[hop] = node
			}
		}
	}
	return paths
}
