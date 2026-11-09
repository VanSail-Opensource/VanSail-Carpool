package subscription

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestEndpointFullHysteriaNativeUsesOnlyOfficialAuthorityAndQueries(t *testing.T) {
	p := hoppingProxy()
	p.Name = "Hysteria"
	p.Stream = map[string]any{"network": "hysteria", "security": "tls", "hysteriaSettings": map[string]any{"auth": "synthetic/auth"}, "tlsSettings": map[string]any{"serverName": "front.example", "alpn": []any{"h3"}}}
	raw, err := p.NativeLink()
	if err != nil || !strings.Contains(raw, "@[2001:db8::1]:443,20000-30000,65535?") || !strings.Contains(raw, "sni=front.example") {
		t.Fatal(raw, err)
	}
	for _, invented := range []string{"mport=", "hopInterval=", "hop-interval=", "security=", "type=", "alpn="} {
		if strings.Contains(raw, invented) {
			t.Fatal("invented Hysteria query", invented, raw)
		}
	}
	compat := endpointClientCompatibility(p, "native")
	if !compat.Supported || !slices.Contains(compat.OmittedOptions, "hysteriaHopInterval") {
		t.Fatal(compat)
	}
}

func TestEndpointNativeTLSUsesDocumentedPinVerificationAndECH(t *testing.T) {
	p := endpointReviewProxy()
	p.Stream["network"] = "tcp"
	pin, verify, ech := strings.Repeat("ab", 32), "leaf.example,alternate.example", "synthetic-ech"
	p.EndpointTuning = &EndpointOptions{TlsPinnedPeerCertSha256: &pin, TlsVerifyPeerCertByName: &verify, TlsEchConfigList: &ech}
	raw, err := p.NativeLink()
	if err != nil {
		t.Fatal(err)
	}
	link, err := url.Parse(raw)
	if err != nil || link.Query().Get("pcs") != pin || link.Query().Get("vcn") != verify || link.Query().Get("ech") != ech {
		t.Fatal(raw, err)
	}
}

func TestEndpointVMessNativeUsesItsOwnJSONTransportContract(t *testing.T) {
	p := endpointReviewProxy()
	p.Protocol = "vmess"
	p.Stream["network"] = "grpc"
	p.Stream["grpcSettings"] = map[string]any{"serviceName": "inherited"}
	service, fingerprint := "administrator-service", "firefox"
	alpn := []string{"h2"}
	p.EndpointTuning = &EndpointOptions{GrpcServiceName: &service, Fingerprint: &fingerprint, TlsAlpn: &alpn}
	raw, err := p.NativeLink()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(raw, "vmess://"))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err = json.Unmarshal(decoded, &fields); err != nil || fields["path"] != service || fields["fp"] != fingerprint || fields["alpn"] != "h2" {
		t.Fatal(fields, err)
	}
}

func TestEndpointRealityNativePasswordAliasCannotOverrideTypedPublicKey(t *testing.T) {
	p := endpointReviewProxy()
	p.Stream["security"] = "reality"
	p.Stream["realitySettings"] = map[string]any{"password": "inherited", "publicKey": "ignored-alias", "serverName": "front.example", "fingerprint": "chrome"}
	key := "typed-current"
	p.EndpointTuning = &EndpointOptions{RealityPublicKey: &key}
	out, err := p.Outbound()
	settings := object(object(out, "streamSettings"), "realitySettings")
	if err != nil || settings["publicKey"] != key || settings["password"] != nil || object(p.Stream, "realitySettings")["password"] != "inherited" {
		t.Fatal(settings, err)
	}
}

func TestEndpointFinalMaskEssentialAndOptionalClientBoundaries(t *testing.T) {
	p := hoppingProxy()
	p.Stream = map[string]any{"network": "hysteria", "security": "tls", "hysteriaSettings": map[string]any{"auth": "synthetic"}, "tlsSettings": map[string]any{"serverName": "front.example"}, "finalmask": map[string]any{"udp": []any{map[string]any{"type": "noise", "settings": map[string]any{}}}}}
	compat := endpointClientCompatibility(p, "clash")
	if !compat.Supported || !slices.Contains(compat.OmittedOptions, "finalmask.noise") {
		t.Fatal("optional one-way noise excluded client", compat)
	}
	object(p.Stream, "finalmask")["udp"] = []any{map[string]any{"type": "salamander", "settings": map[string]any{"password": "synthetic"}}}
	for _, format := range []string{"native", "base64", "sing-box", "clash", "stash"} {
		if endpointClientCompatibility(p, format).Supported {
			t.Fatal("peer-decoded mask silently dropped", format)
		}
	}
	if !endpointClientCompatibility(p, "xray").Supported {
		t.Fatal("native mask support lost")
	}
}
