package xnode

import (
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"slices"
)

// Untracked proxy fallbacks and different sniffing policies can change the
// destination between hops. Those contexts cannot support a disjointness proof.
func routingServerContext(root map[string]any, tags []string, forceSniff, usesProtocol bool) (string, error) {
	unknown := func() error {
		return fmt.Errorf("%w: %w", ErrInvalid, &routingmatch.Diagnostic{Code: "routing_context_unproven"})
	}
	outs, _ := root["outbounds"].([]any)
	safe := map[string]bool{}
	for i, raw := range outs {
		out, _ := raw.(map[string]any)
		protocol, _ := out["protocol"].(string)
		tag, _ := out["tag"].(string)
		allowed := protocol == "freedom" || protocol == "blackhole" || protocol == "dns"
		safe[tag] = allowed
		if i == 0 && !allowed {
			return "", unknown()
		}
	}
	routing, _ := root["routing"].(map[string]any)
	rules, _ := routing["rules"].([]any)
	for _, raw := range rules {
		rule, _ := raw.(map[string]any)
		if _, marker := rule["UserRouting"]; marker {
			continue
		}
		tag, _ := rule["outboundTag"].(string)
		if tag == "" || rule["balancerTag"] != nil || !safe[tag] {
			return "", unknown()
		}
	}
	inbounds, _ := root["inbounds"].([]any)
	context := ""
	seen := 0
	for _, raw := range inbounds {
		inbound, _ := raw.(map[string]any)
		tag, _ := inbound["tag"].(string)
		if !slices.Contains(tags, tag) {
			continue
		}
		value, ok := inbound["sniffing"]
		if !ok {
			value = map[string]any{}
		}
		sniff, _ := value.(map[string]any)
		overrides, _ := sniff["destOverride"].([]any)
		if (forceSniff || sniff["enabled"] == true) && (len(overrides) > 0 || usesProtocol) {
			return "", unknown()
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		if seen > 0 && context != string(encoded) {
			return "", unknown()
		}
		context = string(encoded)
		seen++
	}
	if seen == 0 {
		return "", unknown()
	}
	return context, nil
}
