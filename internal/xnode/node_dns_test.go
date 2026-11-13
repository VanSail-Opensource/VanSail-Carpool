package xnode

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestStoredNodeDNSMigrationPreservesOnlyEffectiveNativeDefault(t *testing.T) {
	for _, test := range []struct {
		raw     string
		applied bool
	}{
		{`{"dns":{"mode":"smartdns","redirect":true,"upstreams":["ignored.invalid"]}}`, true},
		{`{"dns":{"mode":"smartdns","redirect":false}}`, false},
		{`{"dns":{"mode":"xray","redirect":true,"upstreams":["ignored.invalid"]}}`, false},
		{`{}`, false},
		{`{"legacyDnsLoopback":true,"dns":{"redirect":true}}`, true},
		{`{"legacyDnsLoopback":true,"dns":{"redirect":false}}`, false},
		{`{"nativeDnsFallbackServers":["127.0.0.1"],"dns":{"redirect":true}}`, true},
		{`{"nativeDnsFallbackServers":[],"legacyDnsLoopback":true,"dns":{"redirect":true}}`, false},
	} {
		settings, err := decodeNodeSettings([]byte(test.raw))
		if err != nil {
			t.Fatal(err)
		}
		native := map[string]any{}
		applyNodeDNS(native, settings)
		if _, exists := native["dns"]; exists != test.applied {
			t.Fatal("migration invented or erased the resolver", test.raw, native)
		}
		if test.applied && !reflect.DeepEqual(native["dns"].(map[string]any)["servers"], []any{"127.0.0.1"}) {
			t.Fatal("unused upstream list became authoritative", native)
		}
		public, err := json.Marshal(settings)
		if err != nil || strings.Contains(string(public), `"mode"`) || strings.Contains(string(public), "upstreams") || strings.Contains(string(public), "Fallback") || strings.Contains(string(public), "Loopback") {
			t.Fatal("historical or internal state entered the public settings", string(public), err)
		}
	}
	for _, raw := range []string{
		`{"nativeDnsFallbackServers":["203.0.113.53"]}`,
		`{"nativeDnsFallbackServers":["127.0.0.1","127.0.0.1"]}`,
		`{"nativeDnsFallbackServers":"127.0.0.1"}`,
	} {
		if _, err := decodeNodeSettings([]byte(raw)); err == nil {
			t.Fatal("invalid stored native default was accepted", raw)
		}
	}
}

type nodeDNSRepository struct {
	ddnsBindingTestRepository
	configuration Configuration
}

func (r *nodeDNSRepository) XNodeConfiguration(context.Context, string) (Configuration, error) {
	return r.configuration, nil
}

func TestNodeDNSOrdinarySaveCanonicalizesStoredDefaultWithoutChangingSharedConfig(t *testing.T) {
	for _, raw := range []string{
		`{"dns":{"mode":"smartdns","redirect":true}}`,
		`{"legacyDnsLoopback":true,"dns":{"mode":"xray","redirect":true}}`,
		`{"bandwidth":{"normalMbps":100},"dns":{"mode":"smartdns","redirect":true,"upstreams":["ignored.invalid"]}}`,
	} {
		service, source := encryptedNodeSettings(t, raw)
		shared := `{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`
		cipher, err := service.vault.Encrypt("xnode.config.config", shared)
		if err != nil {
			t.Fatal(err)
		}
		repo := &nodeDNSRepository{
			ddnsBindingTestRepository: ddnsBindingTestRepository{nodes: map[string]Node{"node": source.node}, bindings: map[string]DDNSHostBinding{}},
			configuration:             Configuration{ID: "config", DocumentCipher: cipher},
		}
		service.repo = repo
		// A fresh public draft has no access to private saved defaults. Disabling
		// and re-enabling interception must retain the established per-node DNS.
		for _, redirect := range []bool{true, false, true} {
			node := repo.nodes["node"]
			node.Name = "Renamed node"
			saved, err := service.SaveNode(context.Background(), node, NodeSettings{DNS: wire.DNSPolicy{Redirect: redirect}})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := service.vault.Decrypt("xnode.node.node", saved.SettingsCipher)
			if err != nil || !strings.Contains(encoded, `"nativeDnsFallbackServers":["127.0.0.1"]`) || strings.Contains(encoded, "legacyDnsLoopback") || strings.Contains(encoded, `"mode"`) || strings.Contains(encoded, "upstreams") || strings.Contains(encoded, "bandwidth") {
				t.Fatal("ordinary save did not canonicalize node state", encoded, err)
			}
			_, restored, err := service.Node(context.Background(), "node")
			if err != nil || restored.DNS.Redirect != redirect || !reflect.DeepEqual(restored.dnsFallbackServers, []string{"127.0.0.1"}) {
				t.Fatal("saved native default did not round trip", restored, err)
			}
		}
		if retained, err := service.vault.Decrypt("xnode.config.config", repo.configuration.DocumentCipher); err != nil || retained != shared {
			t.Fatal("node migration changed a shared configuration", retained, err)
		}
	}
}

func TestNodeDNSPreservationRejectsUnreadablePriorSettings(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{vault: vault}
	if err = service.preserveNodeDNS(Node{ID: "node", SettingsCipher: "broken"}, &NodeSettings{}); err == nil {
		t.Fatal("unreadable prior settings silently lost stored native defaults")
	}
}

func TestNodeDNSFallbackPreservesExplicitAndMalformedNativeServers(t *testing.T) {
	settings := NodeSettings{dnsFallbackServers: []string{"127.0.0.1"}, DNS: wire.DNSPolicy{Redirect: true}}
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
	settings := NodeSettings{dnsFallbackServers: []string{"127.0.0.1"}, DNS: wire.DNSPolicy{Redirect: true}}
	result, err := nodeDNSDocument(raw, settings)
	if err != nil || string(raw) != original {
		t.Fatal("source document changed", err)
	}
	var native map[string]any
	if json.Unmarshal(result, &native) != nil || native["dns"].(map[string]any)["queryStrategy"] != "UseIPv4" {
		t.Fatal("unrelated DNS fields were lost", string(result))
	}
}
