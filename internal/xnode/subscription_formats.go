package xnode

import (
	"encoding/base64"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func renderSubscription(format string, proxies []subscription.Proxy) ([]byte, error) {
	selected, _, err := subscription.CompatibleProxies(format, proxies)
	if err != nil {
		return nil, err
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
