package xnode

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestStoredDDNSBindingsMigrateOnlyOmittedArrayFields(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want map[string]DDNSHostBinding
	}{
		{`{}`, nil},
		{`{"ddnsBindings":{}}`, map[string]DDNSHostBinding{}},
		{`{"ddnsBindings":{"wan":{"ipv4HostId":"old-v4","ipv6HostId":"old-v6"}}}`, map[string]DDNSHostBinding{"wan": {IPv4HostIDs: []string{"old-v4"}, IPv6HostIDs: []string{"old-v6"}}}},
		{`{"ddnsBindings":{"wan":{"ipv4HostIds":[],"ipv4HostId":"old-v4","ipv6HostIds":["new-v6"],"ipv6HostId":"old-v6"}}}`, map[string]DDNSHostBinding{"wan": {IPv4HostIDs: []string{}, IPv6HostIDs: []string{"new-v6"}}}},
		{`{"ddnsBindings":{"wan":{"ipv4HostIds":null,"ipv4HostId":"old-v4"}}}`, map[string]DDNSHostBinding{"wan": {}}},
	} {
		settings, err := decodeNodeSettings([]byte(test.raw))
		if err != nil || !reflect.DeepEqual(settings.DDNSBindings, test.want) {
			t.Fatal("historical binding meaning changed", test.raw, settings.DDNSBindings, err)
		}
		raw, err := json.Marshal(settings)
		if err != nil || strings.Contains(string(raw), `"ipv4HostId"`) || strings.Contains(string(raw), `"ipv6HostId"`) {
			t.Fatal("retired singleton escaped into current settings", string(raw), err)
		}
		restored, err := decodeNodeSettings(raw)
		if err != nil || len(restored.DDNSBindings) != len(settings.DDNSBindings) {
			t.Fatal("canonical bindings did not round trip", restored, err)
		}
		for id, binding := range settings.DDNSBindings {
			got := restored.DDNSBindings[id]
			if !slices.Equal(got.IPv4HostIDs, binding.IPv4HostIDs) || !slices.Equal(got.IPv6HostIDs, binding.IPv6HostIDs) {
				t.Fatal("canonical host selections changed", got, binding)
			}
		}
	}
	for _, raw := range []string{`{"ddnsBindings":{"wan":{"ipv4HostIds":"invalid"}}}`, `{"ddnsBindings":{"wan":{"ipv6HostIds":[1]}}}`} {
		if _, err := decodeNodeSettings([]byte(raw)); err == nil {
			t.Fatal("malformed stored binding was accepted", raw)
		}
	}
}

func TestStoredNodeSettingsReaderBoundsMigrationInput(t *testing.T) {
	if _, err := decodeNodeSettings([]byte(strings.Repeat(" ", wire.MaxFrameBytes+1))); err == nil {
		t.Fatal("oversized stored settings entered the historical decoder")
	}
}
