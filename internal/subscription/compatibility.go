package subscription

import (
	"errors"
	"slices"
)

// ErrUnsupported identifies a legitimate client capability exclusion.
var ErrUnsupported = errors.New("client does not support this protocol or transport")

// ErrNoCompatibleProxies means selection excluded every available endpoint.
var ErrNoCompatibleProxies = errors.New("no compatible subscription endpoints")

// CompatibleProxies shares format exclusions between metadata, previews and
// downloads. Per-host projection failures omit that host; template/document
// failures are handled separately and never disable compatible peers.
func CompatibleProxies(format string, proxies []Proxy) ([]Proxy, int, error) {
	source := map[string]string{"native": "XRAY_BASE64", "base64": "XRAY_BASE64", "xray": "XRAY_JSON", "sing-box": "SINGBOX", "clash": "MIHOMO", "stash": "STASH"}[format]
	if source == "" {
		return nil, 0, errors.New("unknown subscription format")
	}
	selected, excluded := make([]Proxy, 0, len(proxies)), 0
	for _, proxy := range proxies {
		if slices.Contains(proxy.ExcludedFormats, source) {
			excluded++
			continue
		}
		var err error
		switch format {
		case "native", "base64":
			_, err = proxy.NativeLink()
		case "xray":
			_, err = proxy.Outbound()
		case "sing-box":
			_, err = proxy.SingBox()
		case "clash", "stash":
			_, err = proxy.yamlProxy(format)
		}
		if err != nil {
			excluded++
			continue
		}
		selected = append(selected, proxy)
	}
	if len(selected) == 0 {
		return selected, excluded, ErrNoCompatibleProxies
	}
	return selected, excluded, nil
}
