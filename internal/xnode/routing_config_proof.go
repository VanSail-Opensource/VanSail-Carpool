package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// checkRoutingConfig proves presence in the actual native projection, rather
// than treating an unrelated applied revision as evidence for this graph.
func (s *Service) checkRoutingConfig(ctx context.Context, node string, plan RoutingPlan, config wire.Config) error {
	doc, err := s.routingDocument(plan)
	if err != nil {
		return err
	}
	identity, err := s.repo.XNodeIdentity(ctx, plan.ProxyUserID)
	if err != nil {
		return err
	}
	var native struct {
		Routing struct {
			Rules []map[string]any `json:"rules"`
		} `json:"routing"`
	}
	if err = json.Unmarshal(config.Xray, &native); err != nil {
		return ErrInvalid
	}
	if plan.Phase == "revoking" || plan.Phase == "disabled" {
		choices := slices.Clone(doc.Choices)
		if plan.PreviousCipher != "" {
			previous := plan
			previous.Cipher = plan.PreviousCipher
			old, err := s.routingDocument(previous)
			if err != nil {
				return err
			}
			choices = append(choices, old.Choices...)
		}
		for _, choice := range choices {
			tag, err := managedRoutingTag(identity, choice.ID)
			if err != nil {
				return err
			}
			for _, rule := range native.Routing.Rules {
				if rule["outboundTag"] == tag {
					return fmt.Errorf("routing_revocation_missing: %w", ErrConflict)
				}
			}
		}
		for _, user := range config.Users {
			if len(user.TransitCredentials) == 0 {
				continue
			}
			owner, err := s.repo.XNodeIdentity(ctx, int64(user.ID))
			if err != nil {
				return err
			}
			if owner.MemberOwnerID() == identity.MemberOwnerID() {
				return fmt.Errorf("routing_transit_revocation_missing: %w", ErrConflict)
			}
		}
		return nil
	}
	if !routingDocumentTouchesNode(doc, node) {
		return nil
	}
	in, err := s.routingInput(ctx, identity, doc.Choices)
	if err != nil {
		return err
	}
	if in.dependencies != doc.Dependencies {
		return fmt.Errorf("routing_generation_changed: %w", ErrConflict)
	}
	for _, rule := range in.rules {
		source, target := in.sources[rule.ID], in.targets[rule.ID]
		sourceOwner, targetOwner := in.principals[source.ID], in.principals[target.ID]
		if sourceOwner.ProxyUserID == 0 {
			sourceOwner = identity
		}
		if targetOwner.ProxyUserID == 0 {
			targetOwner = identity
		}
		if rule.From == node {
			tag, err := managedRoutingTag(identity, rule.ID)
			if err != nil {
				return err
			}
			expected := managedNativeRule(rule, sourceOwner.ProxyUserID, tag)
			if !in.legacy {
				expected["inboundTag"] = []string{source.InboundTag}
			}
			found := slices.ContainsFunc(native.Routing.Rules, func(actual map[string]any) bool { return sameRoutingJSON(expected, actual) })
			if !found || !routingUserPresent(config.Users, sourceOwner.ProxyUserID, source.InboundTag) {
				return fmt.Errorf("routing_source_missing node %s rule %s: %w", node, rule.ID, ErrConflict)
			}
		}
		if rule.To == node && (plan.Phase == "active" || plan.Phase == "enabling") {
			_, expected := routingCredentials(doc, targetOwner, routingmatchRule{rule.ID, rule.From, rule.To}, target)
			found := false
			for _, user := range config.Users {
				if int64(user.ID) != targetOwner.ProxyUserID {
					continue
				}
				for _, binding := range user.TransitCredentials {
					found = found || binding.SourceNodeID == expected.SourceNodeID && binding.InboundTag == expected.InboundTag && binding.Generation == expected.Generation && len(binding.Credential) > 0
				}
			}
			if !found {
				return fmt.Errorf("routing_transit_missing node %s rule %s: %w", node, rule.ID, ErrConflict)
			}
		}
	}
	return nil
}

func routingUserPresent(users []wire.User, owner int64, tag string) bool {
	return slices.ContainsFunc(users, func(user wire.User) bool { return int64(user.ID) == owner && slices.Contains(user.InboundTags, tag) })
}

func sameRoutingJSON(a, b any) bool {
	x, err := json.Marshal(a)
	if err != nil {
		return false
	}
	y, err := json.Marshal(b)
	return err == nil && string(x) == string(y)
}
