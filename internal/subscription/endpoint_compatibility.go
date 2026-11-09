package subscription

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

type EndpointClientCompatibility struct {
	Format         string   `json:"format"`
	Supported      bool     `json:"supported"`
	Reasons        []string `json:"reasons"`
	OmittedOptions []string `json:"omittedOptions"`
}

// EndpointCompatibility reports administrator intent independently of member
// ActiveTuning. Unsupported essentials exclude only this endpoint in that format.
func EndpointCompatibility(options EndpointOptions, proxy Proxy) []EndpointClientCompatibility {
	proxy.EndpointTuning = &options
	items := []EndpointClientCompatibility{}
	for _, format := range []string{"native", "base64", "xray", "sing-box", "clash", "stash"} {
		items = append(items, endpointClientCompatibility(proxy, format))
	}
	return items
}

func endpointClientCompatibility(p Proxy, format string) EndpointClientCompatibility {
	item := EndpointClientCompatibility{Format: format, Supported: true, Reasons: []string{}, OmittedOptions: []string{}}
	network := text(p.Stream, "network")
	values := endpointValues(p)
	protocols := []string{"vless", "vmess", "trojan", "shadowsocks", "hysteria"}
	if format == "xray" {
		protocols = append(protocols, "http", "socks")
	}
	if !slices.Contains(protocols, p.Protocol) {
		item.Supported, item.Reasons = false, []string{"protocol"}
	}
	if format == "sing-box" && !slices.Contains([]string{"", "raw", "tcp", "hysteria", "ws", "grpc", "httpupgrade"}, network) || format == "stash" && network == "httpupgrade" {
		item.Supported, item.Reasons = false, []string{"transport"}
	}
	if (format == "native" || format == "base64") && !slices.Contains([]string{"", "raw", "tcp", "hysteria", "ws", "grpc", "httpupgrade", "xhttp"}, network) {
		item.Supported, item.Reasons = false, []string{"transport"}
	}
	if (format == "native" || format == "base64") && p.Protocol == "vmess" && text(p.Stream, "security") == "reality" {
		item.Supported, item.Reasons = false, []string{"security"}
	}
	if !endpointFinalMaskSupported(p, format) {
		item.Supported = false
		item.Reasons = append(item.Reasons, "finalMask")
	}
	if masks := endpointFinalMaskOmissions(p, format); len(masks) > 0 {
		item.OmittedOptions = append(item.OmittedOptions, masks...)
		item.Reasons = append(item.Reasons, "format")
	}
	download := object(effectiveXHTTP(object(p.Stream, "xhttpSettings")), "downloadSettings")
	downloadActive := network == "xhttp" && values["downloadEnabled"] != false && endpointXHTTPMode(p, values) != "stream-one"
	if downNetwork := text(download, "network"); downloadActive && downNetwork != "" && downNetwork != "xhttp" {
		item.Supported = false
		item.Reasons = append(item.Reasons, "downloadTransport")
	}
	if len(download) > 0 && network != "xhttp" {
		item.OmittedOptions = append(item.OmittedOptions, "downloadEnabled")
		item.Reasons = append(item.Reasons, "context")
	} else if len(download) > 0 && endpointXHTTPMode(p, values) == "stream-one" {
		item.OmittedOptions = append(item.OmittedOptions, "downloadEnabled")
		item.Reasons = append(item.Reasons, "dependency")
	}
	if p.Protocol == "hysteria" {
		var err error
		if format == "native" || format == "base64" {
			err = endpointHysteriaNative(p, &url.URL{})
		} else {
			err = endpointHysteria(p, format, map[string]any{})
		}
		if err != nil {
			item.Supported = false
			item.Reasons = append(item.Reasons, "hopping")
		}
	}
	if item.Supported && !baselineEndpointSupported(p, format) {
		item.Supported = false
		item.Reasons = append(item.Reasons, "format")
	}
	for _, spec := range allEndpointSpecs() {
		value, chosen := values[spec.Key]
		if !chosen {
			value, chosen = endpointInheritedValue(p, spec)
		}
		if !chosen || !endpointMeaningful(value) {
			continue
		}
		reason := endpointOptionReason(p, spec, format, value, values)
		if reason == "" {
			continue
		}
		item.OmittedOptions = append(item.OmittedOptions, spec.Key)
		if spec.Essential && reason != "context" && reason != "dependency" && reason != "fixedOff" {
			item.Supported = false
		}
		if !slices.Contains(item.Reasons, reason) {
			item.Reasons = append(item.Reasons, reason)
		}
	}
	if p.Protocol == "hysteria" && endpointHysteriaTimingOmitted(p, format) && !slices.Contains(item.OmittedOptions, "hysteriaHopInterval") {
		item.OmittedOptions = append(item.OmittedOptions, "hysteriaHopInterval")
		if !slices.Contains(item.Reasons, "timing") {
			item.Reasons = append(item.Reasons, "timing")
		}
	}
	return item
}

func endpointOptionReason(p Proxy, spec tuningSpec, format string, value any, values map[string]any) string {
	if reason := endpointDependencyReason(spec, p, value, values); reason != "" {
		return reason
	}
	if (spec.Key == "tlsAllowInsecure" || spec.Key == "downloadTlsAllowInsecure") && value == true {
		return "fixedOff"
	}
	if spec.Key == "hysteriaPorts" {
		return "" // Dedicated bounded client adapters implement this field.
	}
	if spec.Key == "hysteriaHopInterval" {
		if endpointHysteriaTimingOmitted(p, format) {
			return "timing"
		}
		return ""
	}
	if spec.Key == "downloadEnabled" || spec.Key == "downloadSecurity" {
		if format == "xray" || format == "clash" || format == "stash" || format == "native" || format == "base64" {
			return ""
		}
		return "format"
	}
	if (format == "native" || format == "base64") && strings.HasPrefix(spec.Mappings["xray"], "streamSettings.xhttpSettings.") && p.Protocol == "vless" {
		return "" // Official XHTTP extra contains its structured download leg.
	}
	if spec.Mappings[format] == "" {
		return "format"
	}
	if format == "native" || format == "base64" {
		if p.Protocol == "hysteria" && spec.Key == "tlsAlpn" && fmt.Sprint(value) == "[h3]" {
			return "" // The official URI's fixed QUIC transport already uses H3.
		}
		if p.Protocol == "hysteria" && spec.Key != "tlsServerName" && spec.Key != "tlsAllowInsecure" {
			return "format"
		}
		if p.Protocol == "vmess" {
			if slices.Contains([]string{"wsPath", "wsHost", "grpcServiceName", "tlsServerName", "tlsAlpn", "fingerprint", "tlsPinnedPeerCertSha256", "tlsVerifyPeerCertByName"}, spec.Key) {
				return ""
			}
			return "protocol"
		}
		if p.Protocol == "shadowsocks" {
			return "protocol"
		}
		if spec.Key == "tlsAllowInsecure" && p.Protocol != "hysteria" {
			return "format" // No invented generic VLESS/Trojan URI extension.
		}
	}
	if format == "clash" && spec.Key == "xhttpSessionIDTable" && !slices.Contains([]string{"", "uuid", "ALPHABET", "Alphabet", "BASE36", "Base62", "HEX", "alphabet", "base36", "hex", "number"}, fmt.Sprint(value)) {
		return "value"
	}
	if spec.Key == "fingerprint" && !fingerprintSupported(format, fmt.Sprint(value)) {
		return "fingerprint"
	}
	if (format == "sing-box" || format == "clash") && spec.Key == "tcpFastOpen" {
		if _, ok := value.(bool); !ok {
			return "value"
		}
	}
	if (format == "clash" || format == "stash") && spec.Key == "ssNetwork" && value == "udp" {
		return "value"
	}
	if spec.Key == "tcpMptcp" && value == true {
		return "fixedOff"
	}
	return ""
}
