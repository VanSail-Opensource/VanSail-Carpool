package subscription

import (
	"encoding/json"
	"strings"

	xrayconf "github.com/xtls/xray-core/infra/conf"
	"golang.org/x/net/http/httpguts"
)

// ValidateEndpointOptions validates current-inbound applicability without
// applying member dependency pruning or silently deleting administrator intent.
func ValidateEndpointOptions(options EndpointOptions, proxy Proxy) map[string]string {
	values, err := options.values()
	if err != nil {
		return map[string]string{"clientOptions": "invalid"}
	}
	errors := validateTuningValues(values, allEndpointSpecs(), true)
	for _, key := range []string{"tlsAllowInsecure", "downloadTlsAllowInsecure"} {
		if values[key] == true {
			errors[key] = "fixedOff"
		}
	}
	for _, spec := range allEndpointSpecs() {
		if _, ok := values[spec.Key]; !ok {
			continue
		}
		if reason := endpointDependencyReason(spec, proxy, values[spec.Key], values); reason != "" {
			errors[spec.Key] = reason
		}
	}
	mode := endpointXHTTPMode(proxy, values)
	if values["xhttpUplinkHTTPMethod"] == "GET" && mode != "packet-up" {
		errors["xhttpUplinkHTTPMethod"] = "dependency"
	}
	if placement := values["xhttpUplinkDataPlacement"]; (placement == "header" || placement == "cookie") && mode != "packet-up" {
		errors["xhttpUplinkDataPlacement"] = "dependency"
	}
	if mode == "stream-one" {
		for key := range values {
			if strings.HasPrefix(key, "download") && values["downloadEnabled"] != false {
				errors[key] = "dependency"
			}
		}
	}
	if options.HysteriaHopInterval != nil && (options.HysteriaPorts != nil && len(*options.HysteriaPorts) == 0 || options.HysteriaPorts == nil && !hasEndpointHysteriaPorts(proxy)) {
		errors["hysteriaHopInterval"] = "dependency"
	}
	if proxy.Protocol == "hysteria" {
		checked := proxy
		checked.EndpointTuning = &options
		if _, err := endpointHop(checked); err != nil {
			errors["hysteriaPorts"] = "invalidNative"
		}
	}
	for _, prefix := range []string{"", "download"} {
		minKey, maxKey := "tlsMinVersion", "tlsMaxVersion"
		if prefix != "" {
			minKey, maxKey = "downloadTlsMinVersion", "downloadTlsMaxVersion"
		}
		min, minOK := values[minKey].(string)
		max, maxOK := values[maxKey].(string)
		if minOK && maxOK && min > max {
			errors[maxKey] = "conflict"
		}
	}
	if tuningRangePositive(values["downloadXmuxMaxConcurrency"]) && tuningRangePositive(values["downloadXmuxMaxConnections"]) {
		errors["downloadXmuxMaxConnections"] = "conflict"
	}
	if len(errors) != 0 {
		return errors
	}
	proxy.EndpointTuning = &options
	prepared, err := proxy.endpointProjection()
	if download := object(effectiveXHTTP(object(prepared.Stream, "xhttpSettings")), "downloadSettings"); download != nil && text(download, "network") != "xhttp" {
		errors["downloadEnabled"] = "transport"
		return errors
	}
	if err == nil {
		var raw []byte
		raw, err = json.Marshal(prepared.Stream)
		if err == nil {
			var stream xrayconf.StreamConfig
			if err = json.Unmarshal(raw, &stream); err == nil {
				_, err = stream.Build()
			}
		}
	}
	if err != nil {
		errors["clientOptions"] = "invalidNative"
	}
	return errors
}

func validEndpointHeaders(rows []any) bool {
	if len(rows) > 32 {
		return false
	}
	seen := map[string]bool{}
	for _, raw := range rows {
		row := raw.(map[string]any)
		name, value := row["name"].(string), row["value"].(string)
		key := strings.ToLower(name)
		if key == "host" || seen[key] || !httpguts.ValidHeaderFieldName(name) || !httpguts.ValidHeaderFieldValue(value) || len(name) > 128 || len(value) > 4096 {
			return false
		}
		seen[key] = true
	}
	return true
}
