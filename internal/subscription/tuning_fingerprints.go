package subscription

import (
	xraytls "github.com/xtls/xray-core/transport/internet/tls"
	"slices"
)

// Mihomo's GetFingerprint parser, checked against Meta/component/tls/utls.go.
var mihomoFingerprints = []string{"chrome", "firefox", "safari", "ios", "android", "edge", "360", "qq", "random", "chrome120", "firefox120", "safari16", "chrome_psk", "chrome_psk_shuffle", "chrome_padding_psk_shuffle", "chrome_pq", "chrome_pq_psk", "randomized"}

func fingerprintSupported(format, value string) bool {
	switch format {
	case "clash":
		return slices.Contains(mihomoFingerprints, value)
	case "sing-box", "stash":
		return slices.Contains([]string{"chrome", "firefox", "edge", "safari", "360", "qq", "ios", "android", "random", "randomized"}, value)
	default:
		_, preset := xraytls.PresetFingerprints[value]
		_, modern := xraytls.ModernFingerprints[value]
		_, other := xraytls.OtherFingerprints[value]
		return value == "unsafe" || preset || modern || other
	}
}
