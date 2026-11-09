package subscription

import (
	"slices"
	"strings"
)

// EndpointFields exposes only client controls relevant to this current inbound.
// Catalog ownership keeps administrator-only fields outside the member types.
func EndpointFields(proxy Proxy) []TuningField {
	items := []TuningField{}
	for _, spec := range allEndpointSpecs() {
		if !endpointContextAvailable(spec, proxy, endpointValues(proxy)) {
			continue
		}
		field := spec.TuningField
		field.Dependencies = slices.Clone(field.Dependencies)
		for i := range field.Dependencies {
			if field.Dependencies[i].Key == "xhttpMode" {
				field.Dependencies[i].InheritedValues = []string{endpointXHTTPMode(proxy, endpointValues(proxy))}
			}
		}
		available := true
		field.Available = &available
		items = append(items, field)
	}
	return items
}

func endpointXHTTPMode(p Proxy, values map[string]any) string {
	if mode, ok := values["xhttpMode"].(string); ok {
		serverMode := text(object(p.Stream, "xhttpSettings"), "mode")
		if serverMode != "" && serverMode != "auto" && mode != "auto" && mode != serverMode {
			return serverMode
		}
		return mode
	}
	if mode := text(object(p.Stream, "xhttpSettings"), "mode"); mode != "" {
		return mode
	}
	return "auto"
}

func endpointContextAvailable(spec tuningSpec, p Proxy, values map[string]any) bool {
	if strings.HasPrefix(spec.Key, "download") {
		if text(p.Stream, "network") != "xhttp" {
			return false
		}
		security, _ := values["downloadSecurity"].(string)
		if security == "" {
			security = text(object(effectiveXHTTP(object(p.Stream, "xhttpSettings")), "downloadSettings"), "security")
		}
		if security == "" {
			security = text(p.Stream, "security")
		}
		switch {
		case strings.HasPrefix(spec.Key, "downloadTls") && spec.Key != "downloadTlsServerName":
			return security == "tls"
		case strings.HasPrefix(spec.Key, "downloadReality") || spec.Key == "downloadSpiderX":
			return security == "reality"
		case spec.Key == "downloadFingerprint" || spec.Key == "downloadTlsServerName":
			return security == "tls" || security == "reality"
		}
		return true
	}
	switch spec.Context {
	case "ws", "grpc", "httpupgrade":
		return text(p.Stream, "network") == spec.Context
	case "hysteria":
		return p.Protocol == "hysteria"
	}
	return tuningContextAvailable(spec.Context, p)
}

// Xray's extra replaces the flat extras as a whole; host/path/mode stay flat.
func effectiveXHTTP(source map[string]any) map[string]any {
	if extra := object(source, "extra"); extra != nil {
		return extra
	}
	return source
}
