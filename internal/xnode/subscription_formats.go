package xnode

import (
	"encoding/base64"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"slices"
)

func renderSubscription(format string, proxies []subscription.Proxy) ([]byte, error) {
	sourceFormat := map[string]string{"native": "XRAY_BASE64", "base64": "XRAY_BASE64", "xray": "XRAY_JSON", "sing-box": "SINGBOX", "clash": "MIHOMO", "stash": "STASH"}[format]
	if sourceFormat == "" {
		return nil, ErrInvalid
	}
	selected := make([]subscription.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if !slices.Contains(proxy.ExcludedFormats, sourceFormat) {
			selected = append(selected, proxy)
		}
	}
	if len(selected) == 0 {
		return nil, ErrNotFound
	}
	proxies = selected
	switch format {
	case "base64":
		raw, err := subscription.RenderNative(proxies)
		if err != nil {
			return nil, err
		}
		return []byte(base64.StdEncoding.EncodeToString(raw)), nil
	case "native":
		return subscription.RenderNative(proxies)
	case "xray":
		return subscription.RenderXray(proxies)
	case "sing-box":
		return subscription.RenderSingBox(proxies)
	case "stash":
		return subscription.RenderTemplate(subscription.DefaultTemplate("stash"), proxies)
	case "clash":
		return subscription.RenderClash(proxies)
	default:
		return nil, ErrInvalid
	}
}
