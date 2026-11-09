package xnode

import (
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestEndpointXHTTPOverridesRetainInheritedNestedValues(t *testing.T) {
	base := map[string]any{"network": "xhttp", "xhttpSettings": map[string]any{"path": "/inherited", "extra": map[string]any{
		"headers":          map[string]any{"X-Keep": "base", "X-Replace": "base"},
		"xmux":             map[string]any{"maxConnections": 2, "maxConcurrency": 8},
		"downloadSettings": map[string]any{"address": "down.example", "streamSettings": map[string]any{"security": "tls"}},
	}}}
	proxy := subscription.Proxy{Stream: wire.Clone(base)}
	overrides := map[string]any{
		"streamSettings":   map[string]any{"xhttpSettings": map[string]any{"extra": map[string]any{"xmux": map[string]any{"maxConnections": 4}}}},
		"transportOptions": map[string]any{"extra": map[string]any{"headers": map[string]any{"X-Replace": "endpoint"}, "downloadSettings": map[string]any{"port": 8443}}},
		"legacyHost":       map[string]any{"xhttpExtraParams": `{"headers":{"X-Legacy":"legacy"},"xmux":{"hKeepAlivePeriod":30}}`},
	}
	if err := applyEndpointOptions(&proxy, overrides); err != nil {
		t.Fatal(err)
	}
	xhttp := proxy.Stream["xhttpSettings"].(map[string]any)
	extra := xhttp["extra"].(map[string]any)
	headers := extra["headers"].(map[string]any)
	xmux := extra["xmux"].(map[string]any)
	download := extra["downloadSettings"].(map[string]any)
	if xhttp["path"] != "/inherited" || headers["X-Keep"] != "base" || headers["X-Replace"] != "endpoint" || headers["X-Legacy"] != "legacy" ||
		xmux["maxConnections"] != 4 || xmux["maxConcurrency"] != float64(8) || xmux["hKeepAlivePeriod"] != float64(30) ||
		download["address"] != "down.example" || download["port"] != 8443 || download["streamSettings"].(map[string]any)["security"] != "tls" {
		t.Fatal("nested inheritance was discarded", proxy.Stream)
	}
	if base["xhttpSettings"].(map[string]any)["extra"].(map[string]any)["headers"].(map[string]any)["X-Replace"] != "base" {
		t.Fatal("shared inbound changed during endpoint projection")
	}
}

func TestEndpointIntroducingXHTTPExtraPreservesActiveFlatInheritance(t *testing.T) {
	for _, overrides := range []map[string]any{
		{"streamSettings": map[string]any{"xhttpSettings": map[string]any{"extra": map[string]any{"noGRPCHeader": true}}}},
		{"transportOptions": map[string]any{"extra": map[string]any{"noGRPCHeader": true}}},
		{"legacyHost": map[string]any{"xhttpExtraParams": `{"noGRPCHeader":true}`}},
	} {
		proxy := subscription.Proxy{Stream: map[string]any{"network": "xhttp", "xhttpSettings": map[string]any{
			"path": "/inherited", "headers": map[string]any{"X-Keep": "base"}, "xPaddingBytes": "10-20",
		}}}
		if err := applyEndpointOptions(&proxy, overrides); err != nil {
			t.Fatal(err)
		}
		xhttp := proxy.Stream["xhttpSettings"].(map[string]any)
		extra := xhttp["extra"].(map[string]any)
		if extra["headers"].(map[string]any)["X-Keep"] != "base" || extra["xPaddingBytes"] != "10-20" || extra["noGRPCHeader"] != true || xhttp["path"] != "/inherited" {
			t.Fatal("new extra discarded inherited client settings", proxy.Stream)
		}
	}
}
