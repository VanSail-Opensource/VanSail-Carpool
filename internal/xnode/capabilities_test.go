package xnode

import (
	"encoding/json"
	"errors"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestAuxiliaryCapabilityIsRequiredEvenWithoutPaidUsers(t *testing.T) {
	config := wire.Config{Xray: json.RawMessage(`{"inbounds":[{"tag":"utility","protocol":"tunnel"}]}`)}
	if err := configurationCapabilities(config, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("old agent accepted utility listener: %v", err)
	}
	if err := configurationCapabilities(config, []string{"native_auxiliary_inbounds_v1"}); err != nil {
		t.Fatal(err)
	}
	config.Xray = json.RawMessage(`{"inbounds":[]}`)
	if err := configurationCapabilities(config, nil); err != nil {
		t.Fatalf("empty bootstrap unnecessarily requires extension: %v", err)
	}
}

func TestCountryGenerationReferencesRequireStreamCapability(t *testing.T) {
	config := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Firewall: wire.FirewallPolicy{Enabled: true, SourceGeoIP: &wire.SourceGeoIPPolicy{Generation: "country-v1", Reference: true}}}
	if err := configurationCapabilities(config, []string{"source_geoip_firewall_v1"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("old agent accepted streamed country reference: %v", err)
	}
	if err := configurationCapabilities(config, []string{"source_geoip_firewall_v1", "source_geoip_stream_v1"}); err != nil {
		t.Fatalf("stream-capable agent rejected country reference: %v", err)
	}
}
