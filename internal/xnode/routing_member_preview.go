package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"net/netip"
	"strings"
)

func (s *Service) PreviewOwnedRouting(ctx context.Context, user string, choices []RoutingChoice, start string, d routingmatch.Destination, startEndpoint ...string) ([]routingmatch.Path, *routingmatch.Path, error) {
	identity, err := s.repo.XNodeIdentityByUser(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	in, err := s.routingInput(ctx, identity, choices)
	if err != nil {
		return nil, nil, err
	}
	paths, err := in.graph.Paths()
	if err != nil {
		return nil, nil, err
	}
	if start == "" {
		return in.publicPaths(paths), nil, nil
	}
	accessible := false
	for _, node := range in.nodes {
		accessible = accessible || node.ID == start
	}
	if !accessible {
		return nil, nil, ErrInvalid
	}
	if len(startEndpoint) > 0 && startEndpoint[0] != "" {
		endpoint := in.byEndpoint[startEndpoint[0]]
		if endpoint.ID == "" || endpoint.NodeID != start {
			return nil, nil, ErrInvalid
		}
	}
	if len(d.Domain) > 253 || len(d.IP) > 64 {
		return nil, nil, ErrInvalid
	}
	d.Domain = strings.TrimSpace(d.Domain)
	if _, err := netip.ParseAddr(d.IP); err == nil && d.Domain == d.IP {
		d.Domain = ""
	}
	vertex := start
	if !in.legacy {
		endpoint := in.endpoints[start]
		for _, choice := range in.choices {
			if choice.InboundNodeID == start {
				endpoint = in.sources[choice.ID]
				break
			}
		}
		if len(startEndpoint) > 0 && startEndpoint[0] != "" {
			endpoint = in.byEndpoint[startEndpoint[0]]
			if endpoint.ID == "" || endpoint.NodeID != start {
				return nil, nil, ErrInvalid
			}
		}
		vertex = routingVertex(endpoint)
	}
	trace, err := in.graph.Trace(vertex, d)
	publicTrace := in.publicPaths([]routingmatch.Path{trace})[0]
	return in.publicPaths(paths), &publicTrace, err
}
