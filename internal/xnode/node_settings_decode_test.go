package xnode

import (
	"encoding/json"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// A stored node settings document can predate any later field retirement. Read
// compatibility must keep every node operation working while known fields still
// parse, without rejecting the whole document on one unknown historical key.
func TestDecodeNodeSettingsIgnoresRetiredStoredFields(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(map[string]any{
		"legacyDnsLoopback": true,
		"bandwidth":         map[string]any{"downloadMbps": 1000},
		"resolverMode":      "smartdns",
		"firewall":          wire.FirewallPolicy{Enabled: true},
		"certificates":      map[string]wire.Certificate{"primary": {Provider: "acme"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := decodeNodeSettings(raw)
	if err != nil {
		t.Fatalf("decodeNodeSettings(retired fields) = %v", err)
	}
	if !settings.LegacyDNSLoopback || !settings.Firewall.Enabled || settings.Certificates["primary"].Provider != "acme" {
		t.Fatalf("known settings were lost: %+v", settings)
	}
}

func TestDecodeNodeSettingsRejectsMalformedDocuments(t *testing.T) {
	t.Parallel()
	for _, raw := range [][]byte{[]byte(`{`), []byte(`{"firewall":{"enabled":"yes"}}`)} {
		if _, err := decodeNodeSettings(raw); err == nil {
			t.Fatalf("decodeNodeSettings(%s) accepted malformed settings", raw)
		}
	}
}
