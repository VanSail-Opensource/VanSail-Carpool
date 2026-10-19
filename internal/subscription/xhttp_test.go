package subscription

import (
	"encoding/json"
	"testing"

	"github.com/ghodss/yaml"
)

func TestClashAndStashRenderVlessXHTTPRealityAndOmitUnsupportedHosts(t *testing.T) {
	proxy := PreviewProxies()[0]
	proxy.Name = "XHTTP Reality"
	proxy.Stream = map[string]any{
		"network": "xhttp", "security": "reality",
		"realitySettings": map[string]any{"serverName": "cdn.example", "publicKey": "synthetic-public-key", "shortId": "abcd", "fingerprint": "chrome"},
		"xhttpSettings": map[string]any{"path": "/upload", "host": "cdn.example", "mode": "packet-up", "extra": map[string]any{
			"headers": map[string]any{"X-Route": "synthetic"}, "noGRPCHeader": true, "xPaddingBytes": "100-1000", "scMaxEachPostBytes": "1000000",
			"xmux":             map[string]any{"maxConnections": "2-4"},
			"downloadSettings": map[string]any{"address": "download.example", "port": 443, "network": "xhttp", "security": "tls", "tlsSettings": map[string]any{"serverName": "download.example"}, "xhttpSettings": map[string]any{"path": "/download"}},
		}},
	}
	unsupported := proxy
	unsupported.ID, unsupported.Name, unsupported.Protocol = "unsupported", "Unsupported host", "unsupported"
	brokenMapping := proxy
	brokenMapping.ID, brokenMapping.Name = "mapper", "Invalid host mapping"
	brokenMapping.MapperHost = map[string]any{"clientOverrides": map[string]any{"mapper": map[string]any{"mihomo": "invalid", "stash": "invalid"}}}
	for _, format := range []string{"clash", "stash"} {
		t.Run(format, func(t *testing.T) {
			selected, excluded, err := CompatibleProxies(format, []Proxy{proxy, unsupported, brokenMapping})
			if err != nil || len(selected) != 1 || excluded != 2 {
				t.Fatal("host failure blocked a compatible peer", selected, excluded, err)
			}
			raw, err := RenderTemplate(DefaultTemplate(format), []Proxy{proxy, unsupported, brokenMapping})
			if err != nil {
				t.Fatal("VLESS + XHTTP + Reality failed", err)
			}
			data, err := yaml.YAMLToJSON(raw)
			if err != nil {
				t.Fatal(err)
			}
			var root struct {
				Proxies []map[string]any `json:"proxies"`
			}
			if err = json.Unmarshal(data, &root); err != nil || len(root.Proxies) != 1 {
				t.Fatal("unsupported host leaked into output", string(raw), err)
			}
			out := root.Proxies[0]
			options := object(out, "xhttp-opts")
			reality := object(out, "reality-opts")
			if out["network"] != "xhttp" || out["tls"] != true || options["path"] != "/upload" || options["host"] != "cdn.example" || options["mode"] != "packet-up" || options["no-grpc-header"] != true || options["x-padding-bytes"] != "100-1000" || object(options, "headers")["X-Route"] != "synthetic" || reality["public-key"] != "synthetic-public-key" || reality["support-x25519mlkem768"] != true {
				t.Fatal("XHTTP/Reality settings changed", string(raw))
			}
			if object(options, "reuse-settings")["max-connections"] != "2-4" || object(options, "download-settings")["server"] != "download.example" || object(options, "download-settings")["path"] != "/download" {
				t.Fatal("split download or reuse settings were lost", string(raw))
			}
		})
	}
}
