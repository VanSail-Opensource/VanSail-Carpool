package xnode

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func TestEndpointMapperCopiesOnlyClientCredentialsAndTransport(t *testing.T) {
	private := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x47}, 32))
	for _, security := range []string{"tls", "reality"} {
		t.Run(security, func(t *testing.T) {
			inbound := map[string]any{
				"tag": "paid", "protocol": "vless", "port": 443,
				"serverSecret": "server-only-top-level-secret",
				"settings": map[string]any{
					"decryption": "server-decryption-secret",
					"clients":    []any{map[string]any{"id": "another-member-secret"}},
					"accounts":   []any{map[string]any{"pass": "server-account-secret"}},
				},
				"streamSettings": map[string]any{
					"network": "tcp", "security": security,
					"tlsSettings": map[string]any{"serverName": "public.example", "certificates": []any{
						map[string]any{"certificate": []any{"server-certificate"}, "key": []any{"server-key-secret"}},
					}},
				},
			}
			if security == "reality" {
				inbound["streamSettings"].(map[string]any)["realitySettings"] = map[string]any{
					"privateKey": private, "serverNames": []any{"public.example"}, "shortIds": []any{"0123456789abcdef"},
				}
			}
			before, err := json.Marshal(inbound)
			if err != nil {
				t.Fatal(err)
			}
			stream, err := clientStream(inbound)
			if err != nil {
				t.Fatal(err)
			}
			proxy := subscription.Proxy{Protocol: "vless", Account: map[string]any{"id": "current-member", "encryption": "none"}, Stream: stream}
			overrides := map[string]any{"legacyHost": map[string]any{"mapper": map[string]any{"xrayJson": []any{
				map[string]any{"op": "copy", "from": "settings", "to": "settings"},
				map[string]any{"op": "copy", "from": "streamSettings", "to": "streamSettings"},
				map[string]any{"op": "copy", "from": "tag", "to": "tag"},
				map[string]any{"op": "copy", "from": "serverSecret", "to": "unsafeTopLevel"},
				map[string]any{"op": "copy", "from": "$host.metadata.rawInbound", "to": "unsafeWholeInbound"},
			}}}}
			if err := applyEndpointClient(&proxy, overrides, inbound); err != nil {
				t.Fatal(err)
			}
			mapped, err := subscription.ApplyClientMapper(map[string]any{}, proxy.MapperHost, "xrayJson")
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(mapped)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := json.Marshal(proxy.MapperHost["metadata"])
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{private, "server-decryption-secret", "another-member-secret", "server-account-secret", "server-certificate", "server-key-secret", "server-only-top-level-secret", "privateKey", "certificates"} {
				if strings.Contains(string(raw), forbidden) || strings.Contains(string(metadata), forbidden) {
					t.Fatalf("mapper exposed server secret %q", forbidden)
				}
			}
			if mapped["unsafeTopLevel"] != nil || mapped["unsafeWholeInbound"] != nil || mapped["tag"] != "paid" {
				t.Fatal("mapper copied unapproved metadata or lost its public tag", mapped)
			}
			if !strings.Contains(string(raw), "current-member") || !strings.Contains(string(raw), "public.example") || security == "reality" && !strings.Contains(string(raw), "publicKey") {
				t.Fatal("mapper lost client credentials or public transport", string(raw))
			}
			after, err := json.Marshal(inbound)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("client projection modified stored server configuration", err)
			}
		})
	}
}
