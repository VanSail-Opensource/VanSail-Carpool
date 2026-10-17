package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"slices"
	"sort"
)

type routingInput struct {
	graph        *routingmatch.Graph
	rules        []routingmatch.Rule
	endpoints    map[string]Endpoint
	inbounds     map[string]map[string]any
	dependencies string
	nodes        []RoutingNode
}

func (s *Service) routingInput(ctx context.Context, identity Identity, choices []RoutingChoice) (routingInput, error) {
	in := routingInput{endpoints: map[string]Endpoint{}, inbounds: map[string]map[string]any{}, nodes: []RoutingNode{}}
	if identity.UserID == "" || len(choices) > 64 {
		return in, ErrInvalid
	}
	catalog, err := s.RoutingTemplateConfig(ctx)
	if err != nil {
		return in, err
	}
	templates := map[string]RoutingTemplate{}
	for _, t := range catalog.Templates {
		templates[t.ID] = t
	}
	proxies, err := s.MemberProxies(ctx, identity.UserID)
	if err != nil {
		return in, err
	}
	allowed := map[string]bool{}
	for _, p := range proxies {
		allowed[p.ID] = true
	}
	endpoints, err := s.Endpoints(ctx)
	if err != nil {
		return in, err
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].ID < endpoints[j].ID })
	for _, e := range endpoints {
		if !e.Enabled || !allowed[e.ID] {
			continue
		}
		owned, err := s.NativeOwner(ctx, e.NodeID)
		if err != nil {
			return in, err
		}
		if owned {
			if _, exists := in.endpoints[e.NodeID]; !exists {
				in.endpoints[e.NodeID] = e
			}
		}
	}
	for id := range in.endpoints {
		node, err := s.repo.XNodeNode(ctx, id)
		if err != nil {
			return in, err
		}
		in.nodes = append(in.nodes, RoutingNode{ID: id, Name: node.Name})
	}
	sort.Slice(in.nodes, func(i, j int) bool { return in.nodes[i].Name < in.nodes[j].Name })
	for _, choice := range choices {
		if !memberTagPattern.MatchString(choice.ID) || choice.InboundNodeID == choice.OutboundNodeID {
			return in, ErrInvalid
		}
		if in.endpoints[choice.InboundNodeID].ID == "" || in.endpoints[choice.OutboundNodeID].ID == "" {
			return in, fmt.Errorf("%w: routing nodes require current purchased endpoint access", ErrInvalid)
		}
		_, ok := templates[choice.TemplateID]
		if !ok {
			return in, fmt.Errorf("%w: routing template is unavailable", ErrInvalid)
		}
	}
	assetTemplates := catalog.Templates
	if len(choices) == 0 {
		assetTemplates = nil
	}
	domains, assetHash, err := s.routingExpansion(ctx, assetTemplates)
	if err != nil {
		return in, err
	}
	dependencies := []any{assetHash}
	usedNodes := []string{}
	for _, choice := range choices {
		m := templates[choice.TemplateID].Match
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
	matcherContext := ""
	for _, id := range usedNodes {
		e := in.endpoints[id]
		node, settings, err := s.Node(ctx, id)
		if err != nil {
			return in, err
		}
		config, err := s.ResolveConfiguration(ctx, node.ConfigID)
		if err != nil {
			return in, err
		}
		var root map[string]any
		if json.Unmarshal(config.Document, &root) != nil {
			return in, ErrInvalid
		}
		// DNS resolution can change a target between hops. Only AsIs preserves the
		// destination model used to prove conditional graph safety.
		routing, _ := root["routing"].(map[string]any)
		if strategy, _ := routing["domainStrategy"].(string); strategy != "" && strategy != "AsIs" {
			return in, fmt.Errorf("%w: managed routing requires domainStrategy AsIs on %s", ErrInvalid, node.Name)
		}
		tags, _, err := s.eligibleTags(ctx, identity.UserID, id)
		if err != nil {
			return in, err
		}
		context, err := routingServerContext(root, tags)
		if err != nil {
			return in, err
		}
		if matcherContext != "" && matcherContext != context {
			return in, fmt.Errorf("%w: %w", ErrInvalid, &routingmatch.Diagnostic{Code: "routing_context_unproven"})
		}
		matcherContext = context
		items, _ := root["inbounds"].([]any)
		for _, v := range items {
			b, _ := v.(map[string]any)
			if b["tag"] == e.InboundTag {
				in.inbounds[id] = b
				break
			}
		}
		if in.inbounds[id] == nil {
			return in, ErrInvalid
		}
		dependencies = append(dependencies, e, config.Document, config.SnippetIDs, settings.InboundCertificates, settings.Certificates)
	}
	in.graph, err = routingmatch.Build(in.rules, nil)
	if err != nil {
		return in, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if _, err = in.graph.Paths(); err != nil {
		return in, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	raw, err := json.Marshal(dependencies)
	if err != nil {
		return in, err
	}
	hash := sha256.Sum256(raw)
	in.dependencies = hex.EncodeToString(hash[:])
	return in, nil
}
