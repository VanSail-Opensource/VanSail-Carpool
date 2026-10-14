package subscription

import (
	"encoding/json"
	"github.com/ghodss/yaml"
	"net/url"
	"strings"
	"testing"
)

func TestFormatsPreserveCredentialsAndTransport(t *testing.T) {
	proxy := Proxy{ID: "endpoint", Name: "Tokyo fixture", Protocol: "vless", Address: "proxy.example", Port: 443, Account: map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000", "encryption": "none", "flow": "xtls-rprx-vision"}, Stream: map[string]any{"network": "tcp", "security": "reality", "realitySettings": map[string]any{"serverName": "sni.example", "publicKey": "public-key", "shortId": "0123", "fingerprint": "chrome"}}}
	link, err := proxy.NativeLink()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil || parsed.User.Username() != proxy.Account["id"] || parsed.Query().Get("pbk") != "public-key" || parsed.Query().Get("flow") != "xtls-rprx-vision" {
		t.Fatal("native link lost authentication or REALITY")
	}
	for _, render := range []func([]Proxy) ([]byte, error){RenderXray, RenderSingBox, RenderClash} {
		raw, err := render([]Proxy{proxy})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), proxy.Account["id"].(string)) {
			t.Fatal("format lost preserved UUID")
		}
		if !json.Valid(raw) {
			if _, err = yaml.YAMLToJSON(raw); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestShadowsocks2022ServerAndUserKeysRemainComposite(t *testing.T) {
	proxy := Proxy{ID: "ss", Name: "SS-2022", Protocol: "shadowsocks", Address: "proxy.example", Port: 443, Account: map[string]any{"method": "2022-blake3-aes-256-gcm", "password": "server-key:user-key"}, Stream: map[string]any{"network": "tcp", "security": "none"}}
	for _, render := range []func([]Proxy) ([]byte, error){RenderXray, RenderSingBox, RenderClash} {
		raw, err := render([]Proxy{proxy})
		if err != nil || !strings.Contains(string(raw), "server-key:user-key") {
			t.Fatal("SS-2022 key composition changed")
		}
	}
}
