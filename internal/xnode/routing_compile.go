package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) routingDocument(plan RoutingPlan) (RoutingDocument, error) {
	var d RoutingDocument
	err := s.open(fmt.Sprintf("xnode.routes.%d", plan.ProxyUserID), plan.Cipher, &d)
	return d, err
}
func (s *Service) managedRouting(ctx context.Context, node string, users []wire.User) ([]any, []any, map[int64]bool, error) {
	outs, rules := []any{}, []any{}
	owners := map[int64]bool{}
	repo, ok := s.routingRepository()
	if !ok {
		return outs, rules, owners, nil
	}
	plans, err := repo.XNodeRoutingPlans(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	for id, p := range s.routingOverrides {
		found := false
		for i := range plans {
			if plans[i].ProxyUserID == id {
				plans[i] = p
				found = true
			}
		}
		if !found {
			plans = append(plans, p)
		}
	}
	for _, plan := range plans {
		owners[plan.ProxyUserID] = true
		if plan.Phase == "revoking" || plan.Phase == "disabled" {
			continue
		}
		identity, err := s.repo.XNodeIdentity(ctx, plan.ProxyUserID)
		if err != nil {
			return nil, nil, nil, err
		}
		doc, err := s.routingDocument(plan)
		if err != nil {
			return nil, nil, nil, err
		}
		input, err := s.routingInput(ctx, identity, doc.Choices)
		// Invalid entitlement, endpoint, template or geosite changes disable the
		// entire graph. Removing only one rule could expose a shadowed cycle.
		if err != nil || input.dependencies != doc.Dependencies {
			continue
		}
		for _, r := range input.rules {
			e := input.targets[r.ID]
			credentials, binding := routingCredentials(doc, identity, routingmatchRule{r.ID, r.From, r.To}, e)
			if r.To == node && (plan.Phase == "active" || plan.Phase == "enabling") {
				raw, err := inboundCredential(input.endpointInbounds[e.ID], credentials)
				if err != nil {
					return nil, nil, nil, err
				}
				binding.Credential = raw
				for i := range users {
					if int64(users[i].ID) == identity.ProxyUserID {
						found := false
						for _, existing := range users[i].TransitCredentials {
							found = found || existing.SourceNodeID == binding.SourceNodeID && existing.InboundTag == binding.InboundTag
						}
						if !found {
							users[i].TransitCredentials = append(users[i].TransitCredentials, binding)
						}
					}
				}
			}
			if r.From != node {
				continue
			}
			proxy, err := s.endpointProxyCredentials(ctx, e, credentials)
			if err != nil {
				return nil, nil, nil, err
			}
			host := proxy.Address
			proxy.Address, err = s.resolveRoutingPeer(ctx, host)
			if err != nil {
				return nil, nil, nil, err
			}
			out, err := guardedRoutingOutbound(proxy)
			if err != nil {
				return nil, nil, nil, err
			}
			preserveProxyHost(out, host)
			tag, err := managedRoutingTag(identity, r.ID)
			if err != nil {
				return nil, nil, nil, err
			}
			out["tag"] = tag
			outs = append(outs, out)
			rule := managedNativeRule(r, identity.ProxyUserID, tag)
			if !input.legacy {
				rule["inboundTag"] = []string{input.sources[r.ID].InboundTag}
			}
			rules = append(rules, rule)
		}
	}
	return outs, rules, owners, nil
}
func managedNativeRule(r routingmatch.Rule, id int64, tag string) map[string]any {
	raw, _ := json.Marshal(r.Match)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	// Native Xray accepts network as a comma-separated string, not a JSON list.
	if len(r.Match.Networks) > 0 {
		out["network"] = joinRoutingNetwork(r.Match.Networks)
	}
	out["type"], out["outboundTag"], out["user"] = "field", tag, []string{userMatcher(id)}
	return out
}
func joinRoutingNetwork(values []string) string {
	if len(values) == 2 {
		return "tcp,udp"
	}
	return values[0]
}
