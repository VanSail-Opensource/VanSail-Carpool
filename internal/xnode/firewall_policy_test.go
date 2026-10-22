package xnode

import (
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestGlobalFirewallMigrationKeepsCommonFields(t *testing.T) {
	a := wire.FirewallPolicy{BanSeconds: 3600, Rules: map[string]wire.FirewallRule{"closed_port": {Enabled: false, Threshold: 5, WindowSeconds: 600}}}
	b := wire.Clone(a)
	b.Rules["closed_port"] = wire.FirewallRule{Enabled: false, Threshold: 7, WindowSeconds: 600}
	config := MigrateFirewallDetection([]wire.FirewallPolicy{a, b})
	if !config.MigrationNotice || config.BanSeconds != 3600 || config.Rules["closed_port"] != (wire.FirewallRule{Enabled: false, Threshold: 3, WindowSeconds: 600}) {
		t.Fatalf("lost common policy: %+v", config)
	}
}
