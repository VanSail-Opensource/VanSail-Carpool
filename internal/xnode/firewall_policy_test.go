package xnode

import (
	"maps"
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

func TestFirewallInventoryRuleRemainsDisabled(t *testing.T) {
	for _, name := range []string{"default", "missing", "saved-enabled", "migrated"} {
		t.Run(name, func(t *testing.T) {
			var config *FirewallDetectionConfig
			switch name {
			case "missing":
				config = &FirewallDetectionConfig{BanSeconds: 3600}
			case "saved-enabled", "migrated":
				config = DefaultFirewallDetection()
				config.Rules["source_match"] = wire.FirewallRule{Enabled: true, Threshold: 9, WindowSeconds: 600}
				if name == "migrated" {
					config = MigrateFirewallDetection([]wire.FirewallPolicy{{Rules: config.Rules}})
				}
			}
			var before map[string]wire.FirewallRule
			if config != nil {
				before = maps.Clone(config.Rules)
			}
			policy := config.Apply(wire.FirewallPolicy{Enabled: true})
			if err := policy.ValidateDetection(); err != nil {
				t.Fatal(err)
			}
			if _, enabled := policy.Rule("source_match"); enabled {
				t.Fatal("retired inventory rule enabled")
			}
			if _, enabled := policy.Rule("closed_port"); !enabled {
				t.Fatal("active detection rule disabled")
			}
			if config != nil && !maps.Equal(before, config.Rules) {
				t.Fatal("effective policy mutated saved settings")
			}
		})
	}
	if DefaultFirewallDetection().Rules["source_match"].Enabled {
		t.Fatal("retired inventory rule enabled by default")
	}
}
