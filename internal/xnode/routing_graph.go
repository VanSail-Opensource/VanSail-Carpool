package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"slices"
)

type routingInput struct {
	graph            *routingmatch.Graph
	rules            []routingmatch.Rule
	endpoints        map[string]Endpoint
	dependencies     string
	nodes            []RoutingNode
	available        []RoutingEndpoint
	byEndpoint       map[string]Endpoint
	sources          map[string]Endpoint
	targets          map[string]Endpoint
	endpointInbounds map[string]map[string]any
	vertices         map[string]string
	choices          []RoutingChoice
	legacy           bool
}

func (s *Service) routingInput(ctx context.Context, identity Identity, choices []RoutingChoice) (routingInput, error) {
	in := routingInput{endpoints: map[string]Endpoint{}, nodes: []RoutingNode{}}
	if identity.UserID == "" || len(choices) > 64 {
		return in, ErrInvalid
	}
	catalog, err := s.RoutingTemplateConfig(ctx)
	if err != nil {
		return in, err
	}
	global, err := s.Global(ctx)
	if err != nil {
		return in, err
	}
	templates := map[string]RoutingTemplate{}
	for _, t := range catalog.Templates {
		templates[t.ID] = t
	}
	if err := s.loadRoutingEndpoints(ctx, identity, &in); err != nil {
		return in, err
	}
	if err := in.resolveChoices(choices); err != nil {
		return in, err
	}
	var assetTemplates []RoutingTemplate
	usedTemplates := map[string]bool{}
	for _, choice := range choices {
		template, ok := templates[choice.TemplateID]
		if !ok {
			return in, fmt.Errorf("%w: routing template is unavailable", ErrInvalid)
		}
		if !usedTemplates[template.ID] {
			assetTemplates = append(assetTemplates, template)
			usedTemplates[template.ID] = true
		}
	}
	domains, assetHash, err := s.routingExpansion(ctx, assetTemplates)
	if err != nil {
		return in, err
	}
	dependencies := []any{assetHash, global.Torrent.Enabled}
	usedNodes := []string{}
	usesProtocol := false
	for _, choice := range choices {
		m := templates[choice.TemplateID].Match
		usesProtocol = usesProtocol || len(m.Protocols) > 0
		m.Domains = slices.Clone(m.Domains)
		resolved := []string{}
		for _, d := range m.Domains {
			if values, ok := domains[d]; ok {
				resolved = append(resolved, values...)
			} else {
				resolved = append(resolved, d)
			}
		}
		m.Domains = resolved
		in.rules = append(in.rules, routingmatch.Rule{ID: choice.ID, From: choice.InboundNodeID, To: choice.OutboundNodeID, TemplateID: choice.TemplateID, Match: m})
		dependencies = append(dependencies, templates[choice.TemplateID])
		for _, node := range []string{choice.InboundNodeID, choice.OutboundNodeID} {
			if !slices.Contains(usedNodes, node) {
				usedNodes = append(usedNodes, node)
			}
		}
	}
	slices.Sort(usedNodes)
	proofRules := slices.Clone(in.rules)
	if !in.legacy {
		for index := range proofRules {
			proofRules[index].From = routingVertex(in.sources[proofRules[index].ID])
			proofRules[index].To = routingVertex(in.targets[proofRules[index].ID])
		}
	}
	in.graph, err = routingmatch.Build(proofRules, nil)
	if err != nil {
		return in, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if _, err = in.graph.Paths(); err != nil {
		return in, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	dependencies, err = s.prepareRoutingContexts(ctx, identity, &in, usedNodes, global.Torrent.Enabled, usesProtocol, dependencies)
	if err != nil {
		return in, err
	}
	if !in.legacy {
		dependencies = append(dependencies, "endpoint-routing-v1", in.choices)
	}
	raw, err := json.Marshal(dependencies)
	if err != nil {
		return in, err
	}
	hash := sha256.Sum256(raw)
	in.dependencies = hex.EncodeToString(hash[:])
	return in, nil
}
