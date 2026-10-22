package xnode_test

import (
	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestClientLabelsPreserveNativeValidationAndOutput(t *testing.T) {
	for _, tc := range []struct {
		name, document string
		valid, local   bool
	}{
		{"label", `{"remarks":"Member endpoint","inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`, true, false},
		{"local assets", `{"remarks":"Member endpoint","inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}],"routing":{"rules":[{"type":"field","domain":["geosite:youtube"],"outboundTag":"direct"}]}}`, true, true},
		{"invalid protocol", `{"remarks":"Member endpoint","inbounds":[],"outbounds":[{"tag":"direct","protocol":"invalid"}]}`, false, false},
		{"unknown root", `{"remarks":"Member endpoint","unknown":true,"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`, false, false},
		{"null", `null`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := json.RawMessage(tc.document)
			result := xnode.ValidateClientConfigurationDocument(raw)
			if result.Valid != tc.valid || result.NeedsAgentValidation != tc.local {
				t.Fatalf("client validation: %+v", result)
			}
			if string(raw) != tc.document {
				t.Fatal("client validation changed exported metadata")
			}
			if tc.valid && xnode.ValidateConfigurationDocument(raw).Valid {
				t.Fatal("client label weakened the server configuration schema")
			}
		})
	}
}
