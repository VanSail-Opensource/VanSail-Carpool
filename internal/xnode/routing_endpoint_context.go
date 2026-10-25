package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
)

func (s *Service) prepareRoutingContexts(ctx context.Context, identity Identity, in *routingInput, usedNodes []string, torrent, usesProtocol bool, dependencies []any) ([]any, error) {
	matcherContext := ""
	// A DAG cannot loop when metadata changes between listeners. Only graphs
	// with structural cycles need identical, immutable matching inputs.
	stableMetadata := !in.graph.StructurallyAcyclic()
	for _, id := range usedNodes {
		selected := []Endpoint{in.endpoints[id]}
		if !in.legacy {
			selected = nil
			seen := map[string]bool{}
			for _, choice := range in.choices {
				for _, endpoint := range []Endpoint{in.sources[choice.ID], in.targets[choice.ID]} {
					if endpoint.NodeID == id && !seen[endpoint.ID] {
						selected = append(selected, endpoint)
						seen[endpoint.ID] = true
					}
				}
			}
			sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
		}
		node, settings, err := s.Node(ctx, id)
		if err != nil {
			return nil, err
		}
		config, err := s.ResolveConfiguration(ctx, node.ConfigID)
		if err != nil {
			return nil, err
		}
		var root map[string]any
		if json.Unmarshal(config.Document, &root) != nil {
			return nil, ErrInvalid
		}
		routing, _ := root["routing"].(map[string]any)
		if strategy, _ := routing["domainStrategy"].(string); stableMetadata && strategy != "" && strategy != "AsIs" {
			return nil, fmt.Errorf("%w: managed routing requires domainStrategy AsIs on %s", ErrInvalid, node.Name)
		}
		tags, _, err := s.eligibleTags(ctx, identity.UserID, id)
		if err != nil {
			return nil, err
		}
		if !in.legacy {
			tags = nil
			for _, endpoint := range selected {
				if !slices.Contains(tags, endpoint.InboundTag) {
					tags = append(tags, endpoint.InboundTag)
				}
			}
		}
		if stableMetadata {
			context, err := routingServerContext(root, tags, torrent, usesProtocol)
			if err != nil {
				return nil, err
			}
			if matcherContext != "" && matcherContext != context {
				return nil, fmt.Errorf("%w: %w", ErrInvalid, &routingmatch.Diagnostic{Code: "routing_context_unproven"})
			}
			matcherContext = context
		}
		items, _ := root["inbounds"].([]any)
		for _, endpoint := range selected {
			if address := s.ddnsRoutingPeer(endpoint.NodeID, endpoint.Address); address != "" {
				dependencies = append(dependencies, "ddns-origin-v1", endpoint.ID, address)
			}
			for _, item := range items {
				inbound, _ := item.(map[string]any)
				if inbound["tag"] == endpoint.InboundTag {
					in.endpointInbounds[endpoint.ID] = inbound
					break
				}
			}
			if in.endpointInbounds[endpoint.ID] == nil {
				return nil, ErrInvalid
			}
		}
		if in.legacy {
			dependencies = append(dependencies, in.endpoints[id], config.Document, config.SnippetIDs, settings.InboundCertificates, settings.Certificates)
		} else {
			dependencies = append(dependencies, selected, config.Document, config.SnippetIDs, settings.InboundCertificates, settings.Certificates)
		}
	}
	return dependencies, nil
}
