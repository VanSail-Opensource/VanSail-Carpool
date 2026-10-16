package subscription

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func templateProxy(id, name string) Proxy {
	return Proxy{ID: id, Name: name, Protocol: "vless", Address: "proxy.example", Port: 443, Account: map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000", "encryption": "none"}, Stream: map[string]any{"network": "tcp", "security": "none"}}
}

func TestWebSocketHostHeaderSurvivesBothClientRepresentations(t *testing.T) {
	proxy := templateProxy("ws", "WebSocket")
	proxy.Stream = map[string]any{"network": "ws", "security": "none", "wsSettings": map[string]any{"path": "/stream", "host": "edge.example"}}
	sing, err := proxy.SingBox()
	if err != nil {
		t.Fatal(err)
	}
	transport := sing["transport"].(map[string]any)
	if transport["host"] != nil || transport["headers"].(map[string]any)["Host"] != "edge.example" {
		t.Fatalf("wrong WebSocket host: %+v", transport)
	}
	proxy.Stream["wsSettings"] = map[string]any{"path": "/stream", "headers": map[string]any{"Host": "edge.example"}}
	link, err := proxy.NativeLink()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil || parsed.Query().Get("host") != "edge.example" {
		t.Fatalf("native host lost: %s, %v", link, err)
	}
}

func TestXraySubscriptionsKeepOneSelectableConfigurationPerEndpoint(t *testing.T) {
	raw, err := RenderXray([]Proxy{templateProxy("one", "Tokyo"), templateProxy("two", "Osaka")})
	if err != nil {
		t.Fatal(err)
	}
	var configs []map[string]any
	if json.Unmarshal(raw, &configs) != nil || len(configs) != 2 {
		t.Fatalf("not a selectable Xray subscription: %s", raw)
	}
	if configs[0]["remarks"] != "Tokyo" || configs[1]["remarks"] != "Osaka" {
		t.Fatal("endpoint labels lost")
	}
	for _, config := range configs {
		if len(config["outbounds"].([]any)) != 2 {
			t.Fatal("endpoint profiles were mixed")
		}
	}
}

func TestTemplateExpansionPreservesStructuresAndRejectsDanglingReferences(t *testing.T) {
	for _, format := range []string{"xray", "sing-box", "clash"} {
		t.Run(format, func(t *testing.T) {
			if _, err := PreviewTemplate(DefaultTemplate(format)); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, document := range []string{
		`{"outbounds":[]}`,
		`{"outbounds":[{"SubscriptionProxies":{}},{"SubscriptionProxies":{}}]}`,
		`{"outbounds":[{"SubscriptionProxies":{"invalid":true}}]}`,
		`{"outbounds":[{"SubscriptionProxies":{}},{"type":"selector","tag":"choice","outbounds":["missing"]}]}`,
		`{"outbounds":[{"SubscriptionProxies":{}},{"type":"selector","tag":"choice","outbounds":["choice"]}]}`,
	} {
		if _, err := PreviewTemplate(Template{Format: "sing-box", Document: json.RawMessage(document)}); err == nil {
			t.Fatalf("invalid template accepted: %s", document)
		}
	}
}

func TestImportedXrayTemplatePreservesClientSettingsWithoutStringInterpolation(t *testing.T) {
	document, err := ConvertXrayTemplate(json.RawMessage(`{"log":{"loglevel":"warning"},"outbounds":[{"tag":"direct","protocol":"freedom"}],"routing":{"rules":[]},"remarks":"${env:SECRET}"}`))
	if err != nil {
		t.Fatal(err)
	}
	proxy := templateProxy("one", "Tokyo")
	proxy.XrayTemplate = document
	raw, err := RenderXray([]Proxy{proxy})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"loglevel":"warning"`) || strings.Contains(string(raw), "SubscriptionProxies") {
		t.Fatalf("source template not converted: %s", raw)
	}
}

func TestExplicitTLSVerificationChoiceSurvivesClientConversion(t *testing.T) {
	proxy := templateProxy("tls", "TLS")
	proxy.Stream = map[string]any{"network": "tcp", "security": "tls", "tlsSettings": map[string]any{"serverName": "proxy.example", "allowInsecure": true}}
	sing, err := proxy.SingBox()
	if err != nil {
		t.Fatal(err)
	}
	clash, err := proxy.Clash()
	if err != nil {
		t.Fatal(err)
	}
	if sing["tls"].(map[string]any)["insecure"] != true || clash["skip-cert-verify"] != true {
		t.Fatal("explicit TLS choice changed")
	}
}
