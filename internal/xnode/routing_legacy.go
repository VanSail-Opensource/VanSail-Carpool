package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"slices"
	"strings"
)

func (s *Service) migrateLegacyRouting(ctx context.Context) error {
	repo, ok := s.routingRepository()
	if !ok {
		return nil
	}
	items, err := repo.XNodeUnmanagedRoutes(ctx, s.routingRuntime.legacyBefore)
	if err != nil {
		return err
	}
	byOwner := map[int64][]Customization{}
	owners := []int64{}
	for _, item := range items {
		if byOwner[item.ProxyUserID] == nil {
			owners = append(owners, item.ProxyUserID)
		}
		byOwner[item.ProxyUserID] = append(byOwner[item.ProxyUserID], item)
	}
	slices.Sort(owners)
	for _, id := range owners {
		identity, err := s.repo.XNodeIdentity(ctx, id)
		if err != nil {
			return err
		}
		choices, conversionErr := s.convertLegacyChoices(ctx, identity, byOwner[id])
		seed, err := ids.Token(32)
		if err != nil {
			return err
		}
		legacy, err := s.archiveLegacyRouting(ctx, identity)
		if err != nil {
			return err
		}
		doc := RoutingDocument{Choices: choices, Seed: seed}
		for _, item := range byOwner[id] {
			doc.LegacyNodes = append(doc.LegacyNodes, item.NodeID)
		}
		plan := RoutingPlan{ProxyUserID: id, Phase: "revoking", LegacyCipher: legacy}
		input, validationErr := s.routingInput(ctx, identity, choices)
		if conversionErr != nil || validationErr != nil {
			plan.Phase, plan.Reason = "disabled", "routing_legacy_incompatible"
			doc.Choices = []RoutingChoice{}
		} else {
			doc.Dependencies = input.dependencies
		}
		plan.Cipher, err = s.seal(fmt.Sprintf("xnode.routes.%d", id), doc)
		if err != nil {
			return err
		}
		if plan.Phase != "disabled" {
			candidate := plan
			candidate.Phase = "active"
			if err = s.preflightRouting(ctx, candidate, RoutingPlan{}); err != nil {
				return err
			}
		}
		fingerprint := sha256.Sum256([]byte(fmt.Sprintf("routing-conversion-%d", id)))
		_, err = repo.QueueXNodeRouting(ctx, plan, providerops.CreateInput{ActorUserID: identity.UserID, OwnerUserID: identity.UserID, Kind: RoutingOperationKind, IdempotencyKey: fmt.Sprintf("routing-conversion-%d", id), RequestFingerprint: hex.EncodeToString(fingerprint[:]), Items: []providerops.ItemInput{{Key: "routing", TargetType: "xnode_user", TargetID: fmt.Sprint(id)}}}, s.now().UTC())
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) convertLegacyChoices(ctx context.Context, identity Identity, items []Customization) ([]RoutingChoice, error) {
	in, err := s.routingInput(ctx, identity, nil)
	if err != nil {
		return nil, err
	}
	catalog, err := s.RoutingTemplateConfig(ctx)
	if err != nil {
		return nil, err
	}
	targets := map[string]string{}
	for node, e := range in.endpoints {
		proxy, err := s.endpointProxy(ctx, identity, e)
		if err != nil {
			return nil, err
		}
		host := proxy.Address
		proxy.Address, err = s.resolveRoutingPeer(ctx, host)
		if err != nil {
			return nil, err
		}
		out, err := proxy.Outbound()
		if err != nil {
			return nil, err
		}
		preserveProxyHost(out, host)
		delete(out, "tag")
		key, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		if targets[string(key)] != "" {
			return nil, ErrInvalid
		}
		targets[string(key)] = node
	}
	choices := []RoutingChoice{}
	for _, item := range items {
		outs, rules := []map[string]any{}, []map[string]any{}
		if err = s.open("xnode.outbounds."+identity.UserID+"."+item.NodeID, item.OutboundsCipher, &outs); err != nil {
			return nil, err
		}
		if err = s.open("xnode.routing."+identity.UserID+"."+item.NodeID, item.RoutingCipher, &rules); err != nil {
			return nil, err
		}
		outNodes := map[string]string{}
		for _, out := range outs {
			tag, _ := out["tag"].(string)
			delete(out, "tag")
			key, err := json.Marshal(out)
			if err != nil {
				return nil, err
			}
			outNodes[tag] = targets[string(key)]
			if outNodes[tag] == "" {
				return nil, ErrInvalid
			}
		}
		for index, rule := range rules {
			if !allowedFields(rule, "type", "domain", "ip", "port", "network", "protocol", "outboundTag", "user") || rule["type"] != "field" {
				return nil, ErrInvalid
			}
			tag, _ := rule["outboundTag"].(string)
			target := outNodes[tag]
			if target == "" || target == item.NodeID {
				return nil, ErrInvalid
			}
			delete(rule, "type")
			delete(rule, "outboundTag")
			delete(rule, "user")
			if network, ok := rule["network"].(string); ok {
				rule["network"] = strings.Split(network, ",")
			}
			raw, err := json.Marshal(rule)
			if err != nil {
				return nil, err
			}
			var match routingmatch.Match
			if json.Unmarshal(raw, &match) != nil {
				return nil, ErrInvalid
			}
			canonical, _ := json.Marshal(match)
			templateID := ""
			for _, t := range catalog.Templates {
				candidate, _ := json.Marshal(t.Match)
				if string(candidate) == string(canonical) {
					templateID = t.ID
					break
				}
			}
			if templateID == "" {
				return nil, ErrInvalid
			}
			digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", item.NodeID, index)))
			choices = append(choices, RoutingChoice{ID: fmt.Sprintf("legacy-%x", digest[:12]), InboundNodeID: item.NodeID, OutboundNodeID: target, TemplateID: templateID})
		}
	}
	return choices, nil
}
