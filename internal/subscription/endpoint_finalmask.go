package subscription

import "strings"

func endpointFinalMaskSupported(p Proxy, format string) bool {
	if format == "xray" || (format == "native" || format == "base64") && p.Protocol == "vless" {
		return true
	}
	final := object(p.Stream, "finalmask")
	for _, bank := range []string{"tcp", "udp"} {
		for _, item := range array(final, bank) {
			mask, _ := item.(map[string]any)
			if endpointOptionalMask(bank, text(mask, "type")) {
				continue
			}
			if p.Protocol != "hysteria" || bank != "udp" || !strings.EqualFold(text(mask, "type"), "udphop") {
				return false
			}
		}
	}
	return true
}

func endpointOptionalMask(bank, kind string) bool {
	return bank == "udp" && strings.EqualFold(kind, "noise") || bank == "tcp" && strings.EqualFold(kind, "fragment")
}

func endpointFinalMaskOmissions(p Proxy, format string) []string {
	if format == "xray" || (format == "native" || format == "base64") && p.Protocol == "vless" {
		return nil
	}
	var omitted []string
	final := object(p.Stream, "finalmask")
	for _, bank := range []string{"tcp", "udp"} {
		for _, item := range array(final, bank) {
			mask, _ := item.(map[string]any)
			if kind := text(mask, "type"); endpointOptionalMask(bank, kind) {
				omitted = append(omitted, "finalmask."+kind)
			}
		}
	}
	return omitted
}
