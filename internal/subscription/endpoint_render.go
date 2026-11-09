package subscription

import (
	"fmt"
	"net/url"
	"strings"
)

func (p Proxy) endpointDocument(format string) (map[string]any, error) {
	if !endpointClientCompatibility(p, format).Supported {
		return nil, fmt.Errorf("%w: essential endpoint option", ErrUnsupported)
	}
	base, err := p.endpointProjection()
	if err != nil {
		return nil, err
	}
	var out map[string]any
	switch format {
	case "xray":
		out, err = base.Outbound()
	case "sing-box":
		out, err = base.SingBox()
	case "clash", "stash":
		out, err = base.yamlProxy(format)
	}
	if err != nil {
		return nil, err
	}
	if format == "xray" {
		// Typed administrator choices take effect after legacy mapping, before
		// the separate member allowlist is overlaid.
		mapped := p
		mapped.Stream, mapped.Mux = object(out, "streamSettings"), object(out, "mux")
		projected, projectionErr := mapped.endpointProjection()
		if projectionErr != nil {
			return nil, projectionErr
		}
		out["streamSettings"] = projected.Stream
		if len(projected.Mux) > 0 {
			out["mux"] = projected.Mux
		}
	}
	values := endpointValues(p)
	for _, spec := range allEndpointSpecs() {
		value, ok := values[spec.Key]
		path := spec.Mappings[format]
		if !ok || path == "" || !endpointContextAvailable(spec, p, values) || !tuningDependenciesForProxy(spec.TuningField, values, p) || endpointOptionReason(p, spec, format, value, values) != "" {
			continue
		}
		if format == "xray" {
			continue // Canonical stream already contains the deeply merged values.
		}
		if spec.Kind == "range" {
			value = wireTuningRange(value.(map[string]any))
		}
		if (format == "clash" || format == "stash") && spec.Key == "ssNetwork" {
			value = value != "tcp"
		}
		if format == "sing-box" && spec.Key == "ssNetwork" && value == "tcp,udp" {
			delete(out, "network")
			continue
		}
		if format == "sing-box" && spec.Key == "curvePreferences" {
			curves := []string{}
			for _, curve := range value.([]any) {
				curves = append(curves, strings.TrimPrefix(curve.(string), "Curve"))
			}
			value = curves
		}
		if format == "sing-box" && (spec.Key == "grpcIdleTimeout" || spec.Key == "grpcHealthCheckTimeout") {
			value = fmt.Sprintf("%gs", value)
		}
		if spec.Kind == "headers" {
			mergeTuningHeaders(out, path, endpointHeaderMap(value))
		} else {
			applyXMUXChoice(out, path, spec.Key, value, values)
			setTuningPath(out, path, value)
		}
		if format == "sing-box" && spec.Key == "fingerprint" {
			setTuningPath(out, "tls.utls.enabled", true)
		}
	}
	if p.Protocol == "hysteria" {
		if err = endpointHysteria(p, format, out); err != nil {
			return nil, err
		}
	}
	if values["downloadEnabled"] == false && (format == "clash" || format == "stash") {
		delete(object(out, "xhttp-opts"), "download-settings")
	}
	pruneEndpointOmissions(p, format, out)
	// Member values overlay only the unchanged generated member allowlist.
	base.Tuning = p.Tuning
	if base.Tuning == nil {
		base.Tuning = &ConnectionOverrides{}
	}
	return base.applyTuning(format, out)
}

func (p Proxy) endpointNativeLink() (string, error) {
	if !endpointClientCompatibility(p, "native").Supported {
		return "", fmt.Errorf("%w: essential endpoint option", ErrUnsupported)
	}
	base, err := p.endpointProjection()
	if err != nil {
		return "", err
	}
	raw, err := base.NativeLink()
	if err != nil {
		return "", err
	}
	if p.Protocol == "vmess" || p.Protocol == "shadowsocks" {
		return raw, nil
	}
	link, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	query, values := link.Query(), endpointValues(p)
	for _, spec := range allEndpointSpecs() {
		value, present := values[spec.Key]
		if !present {
			value, present = endpointInheritedValue(p, spec)
		}
		reason := endpointOptionReason(p, spec, "native", value, values)
		if key := spec.Mappings["native"]; present && key != "" && reason != "" && reason != "context" && reason != "dependency" && reason != "serverMode" {
			query.Del(key)
		}
	}
	for _, spec := range allEndpointSpecs() {
		value, exists := values[spec.Key]
		key := spec.Mappings["native"]
		if !exists || key == "" || !endpointContextAvailable(spec, p, values) || endpointOptionReason(p, spec, "native", value, values) != "" {
			continue
		}
		if p.Protocol == "hysteria" && key == "alpn" {
			continue // Official Hysteria URIs use implicit H3, with no ALPN query.
		}
		if spec.Kind == "strings" {
			items := []string{}
			for _, item := range value.([]any) {
				items = append(items, item.(string))
			}
			value = strings.Join(items, ",")
		}
		if spec.Key == "grpcMultiMode" {
			value = map[bool]string{true: "multi", false: "gun"}[value.(bool)]
		}
		if spec.Key == "tlsAllowInsecure" {
			value = map[bool]string{true: "1", false: "0"}[value.(bool)]
		}
		query.Set(key, fmt.Sprint(value))
	}
	if p.Tuning != nil && (p.Protocol == "vless" || p.Protocol == "trojan") {
		member, valueErr := p.Tuning.values()
		if valueErr != nil {
			return "", valueErr
		}
		for _, spec := range allTuningSpecs() {
			value, exists := member[spec.Key]
			key := spec.Mappings["native"]
			if exists && key != "" && tuningReason(base, spec, "native", value, member) == "" {
				query.Set(key, fmt.Sprint(value))
			}
		}
	}
	link.RawQuery = query.Encode()
	if err = endpointHysteriaNative(p, link); err != nil {
		return "", err
	}
	return link.String(), nil
}
