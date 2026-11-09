package xnode

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestNodeDNSRetirementPreservesOnlyLegacyEffectiveFallback(t *testing.T) {
	for _, test := range []struct {
		mode             string
		redirect, legacy bool
	}{{"smartdns", true, true}, {"smartdns", false, false}, {"xray", true, false}, {"", false, false}} {
		settings := NodeSettings{DNS: wire.DNSPolicy{Mode: test.mode, Redirect: test.redirect, Upstreams: []string{"ignored.invalid"}}}
		normalizeNodeDNS(&settings)
		if settings.DNS.Mode != "xray" || settings.LegacyDNSLoopback != test.legacy {
			t.Fatal("incorrect compatibility state", settings)
		}
		native := map[string]any{}
		applyNodeDNS(native, settings)
		if _, exists := native["dns"]; exists != test.legacy {
			t.Fatal("retirement invented or erased the resolver", native)
		}
		if test.legacy && !reflect.DeepEqual(native["dns"].(map[string]any)["servers"], []any{"127.0.0.1"}) {
			t.Fatal("unused upstream list became authoritative", native)
		}
	}
}

func TestNodeDNSCompatibilitySurvivesOlderClientSave(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{vault: vault}
	for _, raw := range []string{`{"dns":{"mode":"smartdns","redirect":true}}`, `{"legacyDnsLoopback":true,"dns":{"mode":"xray","redirect":true}}`} {
		cipher, err := vault.Encrypt("xnode.node.node", raw)
		if err != nil {
			t.Fatal(err)
		}
		draft := NodeSettings{DNS: wire.DNSPolicy{Mode: "xray", Redirect: true}}
		if err = service.preserveNodeDNS(Node{ID: "node", SettingsCipher: cipher}, &draft); err != nil || !draft.LegacyDNSLoopback {
			t.Fatal("older client erased encrypted compatibility state", err)
		}
	}
	if err = service.preserveNodeDNS(Node{ID: "node", SettingsCipher: "broken"}, &NodeSettings{}); err == nil {
		t.Fatal("unreadable prior settings silently lost compatibility state")
	}
}

func TestNodeDNSRetirementPreservesExplicitAndMalformedNativeServers(t *testing.T) {
	settings := NodeSettings{LegacyDNSLoopback: true, DNS: wire.DNSPolicy{Mode: "xray", Redirect: true}}
	for _, raw := range []string{`{"dns":{"servers":["https://dns.example/dns-query"],"queryStrategy":"UseIPv4"}}`, `{"dns":{"servers":"invalid"}}`, `{"dns":"invalid"}`} {
		result, err := nodeDNSDocument(json.RawMessage(raw), settings)
		if err != nil {
			t.Fatal(err)
		}
		var original, actual any
		_ = json.Unmarshal([]byte(raw), &original)
		_ = json.Unmarshal(result, &actual)
		if !reflect.DeepEqual(original, actual) {
			t.Fatal("explicit native DNS was rewritten", string(result))
		}
	}
}

func TestNodeDNSFallbackIsConfinedToCompiledCopy(t *testing.T) {
	raw := json.RawMessage(`{"inbounds":[],"dns":{"servers":[],"queryStrategy":"UseIPv4"}}`)
	original := string(raw)
	settings := NodeSettings{LegacyDNSLoopback: true, DNS: wire.DNSPolicy{Redirect: true}}
	result, err := nodeDNSDocument(raw, settings)
	if err != nil || string(raw) != original {
		t.Fatal("source document changed", err)
	}
	var native map[string]any
	if json.Unmarshal(result, &native) != nil || native["dns"].(map[string]any)["queryStrategy"] != "UseIPv4" {
		t.Fatal("unrelated DNS fields were lost", string(result))
	}
}
