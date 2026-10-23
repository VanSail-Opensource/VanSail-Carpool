package subscription

import (
	"slices"
	"strings"
)

type TuningApplicability struct {
	Key               string   `json:"key"`
	Format            string   `json:"format"`
	State             string   `json:"state"`
	AppliedHosts      int      `json:"appliedHosts"`
	IncompatibleHosts int      `json:"incompatibleHosts"`
	Reasons           []string `json:"reasons"`
	Restrictions      []string `json:"restrictions,omitempty"`
}

func TuningCompatibility(overrides ConnectionOverrides, proxies []Proxy) []TuningApplicability {
	overrides = ActiveTuning(overrides)
	values, err := overrides.values()
	if err != nil {
		return nil
	}
	items := []TuningApplicability{}
	for _, format := range []string{"xray", "sing-box", "clash", "stash", "native", "base64"} {
		plain := slices.Clone(proxies)
		for index := range plain {
			plain[index].Tuning = nil
		}
		eligible, _, _ := CompatibleProxies(format, plain)
		for _, spec := range allTuningSpecs() {
			_, chosen := values[spec.Key]
			available := len(proxies) == 0 && spec.Context != "uot" && spec.Context != "sager-mux" || slices.ContainsFunc(proxies, func(p Proxy) bool { return tuningContextAvailable(spec.Context, p) })
			if !chosen && !available {
				continue
			}

			item := TuningApplicability{Key: spec.Key, Format: format, State: "inherited", Reasons: []string{}, Restrictions: tuningRestrictions(spec, format)}
			if spec.Key == "tcpMptcp" {
				item.State = "fixed-off"
				items = append(items, item)
				continue
			}
			if value, exists := values[spec.Key]; exists {
				if spec.Mappings[format] == "" {
					item.State = "unsupported"
					item.Reasons = []string{"format"}
				} else {
					item.State = "endpoint-incompatible"
					for _, proxy := range eligible {
						reason := tuningReason(proxy, spec, format, value, values)
						if reason == "" {
							item.AppliedHosts++
						} else {
							item.IncompatibleHosts++
							if !slices.Contains(item.Reasons, reason) {
								item.Reasons = append(item.Reasons, reason)
							}
						}
					}
					if item.AppliedHosts > 0 {
						item.State = "applied"
					}
				}
			}
			items = append(items, item)
		}
	}
	return items
}

func tuningReason(p Proxy, spec tuningSpec, format string, value any, values map[string]any) string {
	if !tuningDependenciesForProxy(spec.TuningField, values, p) {
		return "dependency"
	}
	if (format == "native" || format == "base64") && p.Protocol != "vless" && p.Protocol != "trojan" {
		return "protocol"
	}
	context := spec.Context
	stream := p.Stream
	security := text(stream, "security")
	switch context {
	case "tls":
		if security != "tls" && security != "reality" {
			return "tls"
		}
		if security == "reality" && value == "unsafe" {
			return "fingerprint"
		}
	case "tls-only":
		if security != "tls" {
			return "tls"
		}
	case "reality":
		if security != "reality" {
			return "reality"
		}
	case "xhttp":
		if text(stream, "network") != "xhttp" {
			return "transport"
		}
	case "shadowsocks", "uot":
		if p.Protocol != "shadowsocks" {
			return "protocol"
		}
		if context == "uot" {
			return "serverCapability"
		}
		if spec.Key == "ssNetwork" && value == "udp" && (format == "clash" || format == "stash") {
			return "value"
		}
	case "sager-mux":
		if text(stream, "network") == "xhttp" {
			return "builtInMultiplex"
		}
		return "serverCapability" // Xray Mux is not SagerNet multiplex.
	case "xray-mux":
		if text(stream, "network") == "xhttp" {
			return "builtInMultiplex"
		}
		if p.Protocol != "vless" && p.Protocol != "vmess" && p.Protocol != "trojan" && p.Protocol != "shadowsocks" {
			return "protocol"
		}
	}
	if spec.Key == "xhttpMode" {
		mode := text(object(stream, "xhttpSettings"), "mode")
		if mode != "" && mode != "auto" && value != "auto" && value != mode {
			return "serverMode"
		}
	}
	if spec.Key == "xhttpScMaxEachPostBytes" {
		settings := object(stream, "xhttpSettings")
		if extra := object(settings, "extra"); extra != nil {
			settings = extra
		}
		if limit := tuningEndpointMaximum(settings["scMaxEachPostBytes"]); limit > 0 && value.(map[string]any)["max"].(float64) > limit {
			return "serverLimit"
		}
	}
	if spec.Key == "fingerprint" && !fingerprintSupported(format, value.(string)) {
		return "fingerprint"
	}
	if (format == "sing-box" || format == "clash") && spec.Key == "tcpFastOpen" {
		if _, ok := value.(bool); !ok {
			return "value"
		}
	}
	if spec.Key == "spiderX" {
		if text, _ := value.(string); text != "" && !strings.HasPrefix(text, "/") {
			return "value"
		}
	}
	return ""
}
