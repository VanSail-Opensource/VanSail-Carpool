package subscription

import "strings"

func endpointDependencyReason(spec tuningSpec, p Proxy, value any, values map[string]any) string {
	if !endpointContextAvailable(spec, p, values) {
		return "context"
	}
	if !tuningDependenciesForProxy(spec.TuningField, values, p) {
		return "dependency"
	}
	if xmuxExplicitConflict(spec.Key, values) {
		return "conflict"
	}
	serverMode := text(object(p.Stream, "xhttpSettings"), "mode")
	if spec.Key == "xhttpMode" && serverMode != "" && serverMode != "auto" && value != "auto" && value != serverMode {
		return "serverMode"
	}
	mode := endpointXHTTPMode(p, values)
	if spec.Key == "xhttpUplinkHTTPMethod" && value == "GET" && mode != "packet-up" {
		return "dependency"
	}
	if spec.Key == "xhttpUplinkDataPlacement" && (value == "header" || value == "cookie") && mode != "packet-up" {
		return "dependency"
	}
	if strings.HasPrefix(spec.Key, "download") {
		if spec.Key == "downloadEnabled" && value == false {
			return ""
		}
		if values["downloadEnabled"] == false || mode == "stream-one" {
			return "dependency"
		}
	}
	return ""
}
