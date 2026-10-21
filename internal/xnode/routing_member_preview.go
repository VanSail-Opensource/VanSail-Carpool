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
	entry := ""
	if len(startEndpoint) > 0 {
		entry = startEndpoint[0]
	}
	endpoint, err := in.traceEntry(start, entry)
	if err != nil {
		return nil, nil, err
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
		vertex = routingVertex(endpoint)
	}
	trace, err := in.graph.Trace(vertex, d)
	publicTrace := in.publicPaths([]routingmatch.Path{trace})[0]
	return in.publicPaths(paths), &publicTrace, err
}

// A reachable endpoint alone is insufficient: a trace must enter an inbound
// that has configured member rules, with the same explicit endpoint identity.
func (in routingInput) traceEntry(start, id string) (Endpoint, error) {
	for _, choice := range in.choices {
		endpoint := in.sources[choice.ID]
		if choice.InboundNodeID == start && endpoint.ID != "" && (id == "" || endpoint.ID == id) {
			return endpoint, nil
		}
	}
	return Endpoint{}, ErrInvalid
}
