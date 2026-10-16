package subscription

import (
	"encoding/json"
	"errors"
)

// PreviewTemplate uses synthetic credentials and performs no network requests.
func PreviewTemplate(template Template) ([]byte, error) {
	proxy := Proxy{ID: "preview", Name: "Preview", Protocol: "vless", Address: "proxy.example", Port: 443, Account: map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000", "encryption": "none"}, Stream: map[string]any{"network": "tcp", "security": "none"}}
	return RenderTemplate(template, []Proxy{proxy})
}

func validateRenderedTemplate(format string, root map[string]any) error {
	key, tag := "outbounds", "tag"
	if format == "clash" {
		key, tag = "proxies", "name"
	}
	items, ok := root[key].([]any)
	if !ok {
		return errors.New("missing proxy array")
	}
	known := map[string]bool{}
	if format == "clash" {
		known["DIRECT"], known["REJECT"], known["REJECT-DROP"] = true, true, true
		known["PASS"], known["GLOBAL"] = true, true
	}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return errors.New("invalid proxy entry")
		}
		discriminator := "type"
		if format == "xray" {
			discriminator = "protocol"
		}
		if kind, ok := item[discriminator].(string); !ok || kind == "" {
			return errors.New("proxy protocol/type is required")
		}
		name, ok := item[tag].(string)
		if !ok || name == "" || known[name] {
			return errors.New("proxy names must be unique")
		}
		known[name] = true
	}
	if format == "xray" {
		if routing, ok := root["routing"].(map[string]any); ok {
			if rules, ok := routing["rules"].([]any); ok {
				for _, raw := range rules {
					if rule, ok := raw.(map[string]any); ok {
						if tag, ok := rule["outboundTag"].(string); ok && !known[tag] {
							return errors.New("rule references an unknown outbound")
						}
					}
				}
			}
		}
		return nil
	}
	groups := items
	if format == "clash" {
		groups, _ = root["proxy-groups"].([]any)
		for _, raw := range groups {
			group, ok := raw.(map[string]any)
			if !ok {
				return errors.New("invalid proxy group")
			}
			name, ok := group["name"].(string)
			if !ok || name == "" || known[name] {
				return errors.New("proxy group names must be unique")
			}
			known[name] = true
		}
	}
	for _, raw := range groups {
		group, ok := raw.(map[string]any)
		if !ok {
			return errors.New("invalid outbound")
		}
		refsKey := "outbounds"
		if format == "clash" {
			refsKey = "proxies"
		}
		if refs, exists := group[refsKey]; exists {
			list, ok := refs.([]any)
			if !ok {
				return errors.New("invalid selector entries")
			}
			for _, raw := range list {
				name, ok := raw.(string)
				if !ok || !known[name] {
					return errors.New("selector references an unknown proxy")
				}
			}
		}
	}
	if format == "sing-box" {
		if route, ok := root["route"].(map[string]any); ok {
			if final, ok := route["final"].(string); ok && !known[final] {
				return errors.New("route final references an unknown outbound")
			}
		}
	}
	if format == "clash" {
		if err := validateSelectorCycles(groups, "name", "proxies"); err != nil {
			return err
		}
		return validateClashRules(root, known)
	}
	if err := validateSelectorCycles(groups, "tag", "outbounds"); err != nil {
		return err
	}
	return nil
}

// ConvertXrayTemplate preserves upstream client settings while reserving the
// first outbound for the endpoint, matching the source JSON subscription model.
func ConvertXrayTemplate(raw json.RawMessage) (json.RawMessage, error) {
	var encoded string
	if json.Unmarshal(raw, &encoded) == nil {
		raw = json.RawMessage(encoded)
	}
	var root map[string]any
	if json.Unmarshal(raw, &root) != nil || root == nil {
		return nil, errors.New("invalid source Xray template")
	}
	outs, ok := root["outbounds"].([]any)
	if !ok {
		return nil, errors.New("missing source outbounds")
	}
	root["outbounds"] = append([]any{map[string]any{"SubscriptionProxies": map[string]any{}}}, outs...)
	document, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	if _, err = PreviewTemplate(Template{Format: "xray", Document: document}); err != nil {
		return nil, err
	}
	return document, nil
}
