package subscription

import (
	"encoding/json"
	"fmt"
	"slices"
)

func tuningDependenciesMet(field TuningField, values map[string]any) bool {
	for _, rule := range field.Dependencies {
		value := fmt.Sprint(values[rule.Key])
		if !tuningDependencyMet(rule, value) {
			return false
		}
	}
	return true
}

func tuningDependencyMet(rule TuningDependency, value string) bool {
	return (len(rule.Values) == 0 || slices.Contains(rule.Values, value)) && !slices.Contains(rule.NotValues, value)
}

func tuningDependenciesForProxy(field TuningField, values map[string]any, p Proxy) bool {
	for _, rule := range field.Dependencies {
		value, exists := values[rule.Key]
		if !exists && rule.Key == "xhttpMode" {
			value = text(object(p.Stream, "xhttpSettings"), "mode")
		}
		if !tuningDependencyMet(rule, fmt.Sprint(value)) {
			return false
		}
	}
	return true
}

// ActiveTuning drops inactive dependent values before validation and persistence.
// The editor keeps its complete in-memory draft when a control is hidden.
func ActiveTuning(input ConnectionOverrides) ConnectionOverrides {
	values, err := input.values()
	if err != nil {
		return input
	}
	for _, spec := range allTuningSpecs() {
		if !tuningDependenciesMet(spec.TuningField, values) {
			delete(values, spec.Key)
		}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return input
	}
	var result ConnectionOverrides
	if json.Unmarshal(raw, &result) != nil {
		return input
	}
	return result
}

func tuningContextAvailable(context string, proxy Proxy) bool {
	stream := proxy.Stream
	security := text(stream, "security")
	switch context {
	case "tls":
		return security == "tls" || security == "reality"
	case "tls-only":
		return security == "tls"
	case "reality":
		return security == "reality"
	case "xhttp":
		return text(stream, "network") == "xhttp"
	case "shadowsocks":
		return proxy.Protocol == "shadowsocks"
	case "uot", "sager-mux":
		return false
	case "xray-mux":
		return text(stream, "network") != "xhttp" && slices.Contains([]string{"vless", "vmess", "trojan", "shadowsocks"}, proxy.Protocol)
	}
	return true
}

// TuningFieldsFor shares endpoint applicability with the editor without secrets.
func TuningFieldsFor(proxies []Proxy) []TuningField {
	fields := TuningFields()
	for i := range fields {
		fields[i].Dependencies = slices.Clone(fields[i].Dependencies)
		for j := range fields[i].Dependencies {
			rule := &fields[i].Dependencies[j]
			if rule.Key != "xhttpMode" {
				continue
			}
			for _, p := range proxies {
				if text(p.Stream, "network") != "xhttp" || !tuningContextAvailable(fields[i].Context, p) {
					continue
				}
				mode := text(object(p.Stream, "xhttpSettings"), "mode")
				if mode == "" {
					mode = "auto"
				}
				if !slices.Contains(rule.InheritedValues, mode) {
					rule.InheritedValues = append(rule.InheritedValues, mode)
				}
			}
		}
		available := len(proxies) == 0 && fields[i].Context != "uot" && fields[i].Context != "sager-mux" || slices.ContainsFunc(proxies, func(p Proxy) bool { return tuningContextAvailable(fields[i].Context, p) })
		fields[i].Available = &available
	}
	return fields
}

// ActiveTuningFor applies shared context/dependency rules before owner validation
// and persistence. A temporary disconnection does not affect this host inventory.
func ActiveTuningFor(input ConnectionOverrides, proxies []Proxy) ConnectionOverrides {
	if len(proxies) == 0 {
		return ActiveTuning(input)
	}
	values, err := input.values()
	if err != nil {
		return input
	}
	for _, spec := range allTuningSpecs() {
		if !slices.ContainsFunc(proxies, func(p Proxy) bool {
			if !tuningContextAvailable(spec.Context, p) {
				return false
			}
			return tuningDependenciesForProxy(spec.TuningField, values, p)
		}) {
			delete(values, spec.Key)
		}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return input
	}
	var result ConnectionOverrides
	if json.Unmarshal(raw, &result) != nil {
		return input
	}
	return result
}
