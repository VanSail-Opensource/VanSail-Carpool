package subscription

import (
	"encoding/json"
	"strings"
)

func (p Proxy) endpointProjection() (Proxy, error) {
	base := p
	base.EndpointTuning, base.Tuning = nil, nil
	raw, err := json.Marshal(p.Stream)
	if err != nil {
		return base, err
	}
	base.Stream = map[string]any{}
	if len(raw) != 0 && string(raw) != "null" {
		if err = json.Unmarshal(raw, &base.Stream); err != nil {
			return base, err
		}
	}
	normalizeEndpointReality(base.Stream)
	root := map[string]any{"streamSettings": base.Stream, "mux": cloneTuningObject(p.Mux)}
	values := endpointValues(p)
	for _, spec := range allEndpointSpecs() {
		value, exists := values[spec.Key]
		if !exists || endpointDependencyReason(spec, p, value, values) != "" {
			continue
		}
		path := endpointXrayPath(spec.Mappings["xray"], base, values)
		if path == "" {
			continue
		}
		if strings.HasPrefix(path, "streamSettings.xhttpSettings.extra.") {
			preserveXHTTPExtra(root)
		}
		if strings.HasPrefix(spec.Key, "download") {
			if values["downloadEnabled"] == false {
				continue
			}
			ensureEndpointDownload(root, p, values)
		}
		if spec.Kind == "range" {
			value = wireTuningRange(value.(map[string]any))
		}
		if spec.Kind == "headers" {
			mergeTuningHeaders(root, path, endpointHeaderMap(value))
		} else {
			applyXMUXChoice(root, path, spec.Key, value, values)
			setTuningPath(root, path, value)
		}
	}
	if text(p.Stream, "network") == "xhttp" && endpointXHTTPMode(p, values) != "stream-one" && values["downloadEnabled"] == true {
		preserveXHTTPExtra(root)
		ensureEndpointDownload(root, p, values)
	}
	if text(p.Stream, "network") == "xhttp" && values["downloadEnabled"] == false {
		delete(effectiveXHTTP(object(base.Stream, "xhttpSettings")), "downloadSettings")
	}
	if text(p.Stream, "network") == "xhttp" {
		extra := effectiveXHTTP(object(base.Stream, "xhttpSettings"))
		if endpointXHTTPMode(p, values) == "stream-one" {
			delete(extra, "downloadSettings")
		} else if object(extra, "downloadSettings") != nil {
			ensureEndpointDownload(root, p, values)
		}
	}
	if p.Protocol == "hysteria" {
		if err = endpointHysteria(p, "xray", root); err != nil {
			return base, err
		}
		base.Stream = object(root, "streamSettings")
	}
	base.Mux = object(root, "mux")
	enforceEndpointTLS(base.Stream)
	enforceClientStream(base.Stream)
	return base, nil
}

func endpointHeaderMap(value any) map[string]any {
	headers := map[string]any{}
	for _, raw := range value.([]any) {
		row := raw.(map[string]any)
		headers[row["name"].(string)] = row["value"]
	}
	return headers
}

func endpointXrayPath(path string, p Proxy, values map[string]any) string {
	security := text(p.Stream, "security")
	if strings.Contains(path, "downloadSettings.$security") {
		security, _ = values["downloadSecurity"].(string)
		if security == "" {
			security = text(object(effectiveXHTTP(object(p.Stream, "xhttpSettings")), "downloadSettings"), "security")
		}
		if security == "" {
			security = text(p.Stream, "security")
		}
	}
	return strings.ReplaceAll(path, "$security", security+"Settings")
}

func normalizeEndpointReality(stream map[string]any) {
	settings := object(stream, "realitySettings")
	if password := text(settings, "password"); password != "" {
		settings["publicKey"] = password // Native password takes precedence over its publicKey alias.
		delete(settings, "password")
	}
	download := object(effectiveXHTTP(object(stream, "xhttpSettings")), "downloadSettings")
	if settings = object(download, "realitySettings"); settings != nil {
		if password := text(settings, "password"); password != "" {
			settings["publicKey"] = password
			delete(settings, "password")
		}
	}
}

// The pinned core removed allowInsecure. Legacy encrypted documents remain
// intact; every client projection follows the same secure false policy.
func enforceEndpointTLS(stream map[string]any) {
	if settings := object(stream, "tlsSettings"); settings["allowInsecure"] == true {
		settings["allowInsecure"] = false
	}
	download := object(effectiveXHTTP(object(stream, "xhttpSettings")), "downloadSettings")
	if settings := object(download, "tlsSettings"); settings["allowInsecure"] == true {
		settings["allowInsecure"] = false
	}
}
