package xnode

import (
	"encoding/json"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFreedomFinalRuleValidation(t *testing.T) {
	cases := []struct {
		name  string
		rule  string
		valid bool
	}{
		{"allow", `{"action":"allow"}`, true},
		{"allow with legacy delay", `{"action":"allow","blockDelay":"1-2"}`, false},
		{"allow with log level", `{"action":"allow","logLevel":"info"}`, false},
		{"block with legacy delay", `{"action":"block","blockDelay":"1-2"}`, true},
		{"block with delay and log level", `{"action":"block","delay":"1-2","logLevel":"info"}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conf := wire.Config{Xray: json.RawMessage(
				`{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom","settings":{"finalRules":[` +
					tc.rule + `]}}]}`,
			)}
			if result := ValidateCompiled(conf); result.Valid != tc.valid {
				t.Fatalf("validation result = %+v, want valid=%t", result, tc.valid)
			}
		})
	}
}
