package xnode

import "testing"

// A stored node settings document can predate any later field retirement. Read
// compatibility must keep every node operation working while known fields still
// parse, without rejecting the whole document on one unknown historical key or
// one unknown nested key.
func TestDecodeNodeSettingsIgnoresRetiredStoredFields(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"legacyDnsLoopback": true,
		"bandwidth": {"normalMbps": 1000},
		"resolverMode": "smartdns",
		"future": true,
		"dns": {"mode": "xray", "future": true},
		"firewall": {"enabled": true},
		"certificates": {"primary": {"provider": "acme"}}
	}`)
	settings, err := decodeNodeSettings(raw)
	if err != nil {
		t.Fatalf("decodeNodeSettings(retired fields) = %v", err)
	}
	if len(settings.dnsFallbackServers) != 1 || settings.dnsFallbackServers[0] != "127.0.0.1" || !settings.Firewall.Enabled ||
		settings.Certificates["primary"].Provider != "acme" {
		t.Fatalf("known settings were lost: %+v", settings)
	}
}

func TestDecodeNodeSettingsRejectsMalformedAndNonObjectDocuments(t *testing.T) {
	t.Parallel()
	for _, raw := range [][]byte{
		[]byte(``), []byte(` `), []byte(`{`), []byte(`{} {}`),
		[]byte(`null`), []byte(`[]`), []byte(`"settings"`), []byte(`0`),
		[]byte(`{"firewall":{"enabled":"yes"}}`),
	} {
		if _, err := decodeNodeSettings(raw); err == nil {
			t.Fatalf("decodeNodeSettings(%s) accepted malformed settings", raw)
		}
	}
}
