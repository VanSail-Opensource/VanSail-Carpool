package xnode

import (
	"encoding/json"
	"errors"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestCertificateModeMatchesNativeInferenceWithoutRewritingSettings(t *testing.T) {
	cert, key := certificateFormMaterial(t)
	for _, test := range []struct {
		name   string
		config wire.CertConfig
		mode   string
		valid  bool
	}{
		{"inferred PEM pair", wire.CertConfig{CertContent: cert, KeyContent: key}, "content", true},
		{"PEM pair precedes file pair", wire.CertConfig{CertContent: cert, KeyContent: key, CertFile: "cert.pem", KeyFile: "key.pem"}, "content", true},
		{"inferred file pair", wire.CertConfig{CertFile: "cert.pem", KeyFile: "key.pem"}, "file", true},
		{"file pair with stale certificate PEM", wire.CertConfig{CertFile: "cert.pem", KeyFile: "key.pem", CertContent: "stale"}, "file", true},
		{"file pair with stale private key PEM", wire.CertConfig{CertFile: "cert.pem", KeyFile: "key.pem", KeyContent: "stale"}, "file", true},
		{"PEM pair with stale file path", wire.CertConfig{CertContent: cert, KeyContent: key, CertFile: "stale.pem"}, "content", true},
		{"automatic HTTPS precedes pairs", wire.CertConfig{AutoTLS: true, Domain: "proxy.example", CertContent: cert, KeyContent: key, CertFile: "cert.pem", KeyFile: "key.pem"}, "http", true},
		{"explicit file ignores automatic HTTPS", wire.CertConfig{CertMode: " \tFiLe\n", AutoTLS: true, CertFile: "cert.pem", KeyFile: "key.pem"}, "file", true},
		{"explicit PEM", wire.CertConfig{CertMode: " CONTENT ", CertContent: cert, KeyContent: key}, "content", true},
		{"explicit HTTP", wire.CertConfig{CertMode: " HTTP ", Domain: "proxy.example"}, "http", true},
		{"explicit DNS", wire.CertConfig{CertMode: " DnS ", Domain: "*.proxy.example", DNSProvider: "cloudflare"}, "dns", true},
		{"explicit self", wire.CertConfig{CertMode: " SELF ", Domain: "local.example"}, "self", true},
		{"blank mode infers file pair", wire.CertConfig{CertMode: " \t\n", CertFile: "cert.pem", KeyFile: "key.pem"}, "file", true},
		{"partial certificate PEM", wire.CertConfig{CertContent: cert}, "none", false},
		{"partial private key PEM", wire.CertConfig{KeyContent: key}, "none", false},
		{"partial certificate file", wire.CertConfig{CertFile: "cert.pem"}, "none", false},
		{"partial private key file", wire.CertConfig{KeyFile: "key.pem"}, "none", false},
		{"mixed partial pairs", wire.CertConfig{CertContent: cert, KeyFile: "key.pem"}, "none", false},
		{"no material", wire.CertConfig{}, "none", false},
		{"explicit none stays invalid for managed profiles", wire.CertConfig{CertMode: " NONE "}, "none", false},
		{"explicit file requires both paths", wire.CertConfig{CertMode: " FILE ", CertFile: "cert.pem"}, "file", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := test.config
			before, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			if mode := certificateMode(config); mode != test.mode {
				t.Fatalf("mode %q, want %q", mode, test.mode)
			}
			settings := NodeSettings{Certificates: map[string]wire.Certificate{"primary": {Config: config}}}
			if err = validateCertificateSettings(settings, nil); (err == nil) != test.valid {
				t.Fatalf("validation error %v, want valid=%v", err, test.valid)
			}
			after, err := json.Marshal(settings.Certificates["primary"].Config)
			if err != nil || string(before) != string(after) {
				t.Fatal("mode inference rewrote saved configuration", err)
			}
		})
	}
}

func TestInferredAndNormalizedFileModesRequireNodeFileCapability(t *testing.T) {
	for _, profile := range []wire.CertConfig{
		{CertFile: "cert.pem", KeyFile: "key.pem"},
		{CertMode: " FiLe ", CertFile: "cert.pem", KeyFile: "key.pem"},
		{CertFile: "cert.pem", KeyFile: "key.pem", CertContent: "stale"},
		{CertFile: "cert.pem", KeyFile: "key.pem", KeyContent: "stale"},
	} {
		config := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Certificates: map[string]wire.Certificate{"primary": {Config: profile}}}
		var capability *CapabilityError
		if err := configurationCapabilities(config, nil); !errors.As(err, &capability) || capability.Capability != "node_file_certificates_v1" {
			t.Fatalf("old agent accepted inferred node files: %v", err)
		}
		if err := configurationCapabilities(config, []string{"node_file_certificates_v1"}); err != nil {
			t.Fatal("file-capable agent rejected inferred files", err)
		}
	}
	for _, profile := range []wire.CertConfig{
		{CertFile: "cert.pem", KeyFile: "key.pem", Domain: "proxy.example"},
		{CertContent: "cert", KeyContent: "key", CertFile: "cert.pem", KeyFile: "key.pem"},
		{AutoTLS: true, Domain: "proxy.example", CertFile: "cert.pem", KeyFile: "key.pem"},
	} {
		config := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Certificates: map[string]wire.Certificate{"primary": {Config: profile}}}
		if err := configurationCapabilities(config, nil); err != nil {
			t.Fatal("legacy-compatible or non-file mode unnecessarily requires node files", err)
		}
	}
}
