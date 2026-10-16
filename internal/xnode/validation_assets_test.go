package xnode_test

import (
	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestNodeLocalGeodataValidationCoversDNSAndSourceMatchers(t *testing.T) {
	raw := json.RawMessage(`{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}],"dns":{"servers":[{"address":"1.1.1.1","domains":["geosite:youtube"],"expectedIPs":["geoip:cn"]}]},"routing":{"rules":[{"type":"field","sourceIP":["geoip:cn"],"outboundTag":"direct"},{"type":"field","domain":["geosite:youtube"],"outboundTag":"direct"}]}}`)
	before := string(raw)
	for _, result := range []xnode.ValidationResult{
		xnode.ValidateConfigurationDocument(raw),
		xnode.ValidateCompiled(wire.Config{Xray: raw, Users: []wire.User{}}),
	} {
		if !result.Valid || !result.NeedsAgentValidation {
			t.Fatalf("node-local matchers were not deferred to the agent: %+v", result)
		}
	}
	if string(raw) != before {
		t.Fatal("validation changed the administrator document")
	}
	var invalid map[string]any
	if err := json.Unmarshal(raw, &invalid); err != nil {
		t.Fatal(err)
	}
	invalid["outbounds"] = []any{map[string]any{"tag": "direct", "protocol": "not-a-protocol"}}
	bad, err := json.Marshal(invalid)
	if err != nil {
		t.Fatal(err)
	}
	if result := xnode.ValidateConfigurationDocument(bad); result.Valid {
		t.Fatal("geodata substitution hid an unrelated invalid protocol")
	}
}
