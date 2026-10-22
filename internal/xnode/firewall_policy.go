package xnode

import (
	"maps"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// FirewallDetectionConfig is the fleet-wide source of detection decisions.
type FirewallDetectionConfig struct {
	Rules           map[string]wire.FirewallRule `json:"rules"`
	BanSeconds      int                          `json:"banSeconds"`
	MigrationNotice bool                         `json:"migrationNotice"`
}

func DefaultFirewallDetection() *FirewallDetectionConfig {
	config := &FirewallDetectionConfig{Rules: map[string]wire.FirewallRule{}, BanSeconds: 259200}
	for _, name := range wire.FirewallRuleNames() {
		config.Rules[name] = wire.FirewallRule{Enabled: true, Threshold: 3, WindowSeconds: 86400}
	}
	return config
}

func (config *FirewallDetectionConfig) Apply(policy wire.FirewallPolicy) wire.FirewallPolicy {
	if config == nil {
		config = DefaultFirewallDetection()
	}
	policy.Rules = maps.Clone(config.Rules)
	policy.BanSeconds = config.BanSeconds
	return policy
}

// MigrateFirewallDetection preserves each common value and uses protocol
// defaults only for fields that disagree between saved node policies.
func MigrateFirewallDetection(policies []wire.FirewallPolicy) *FirewallDetectionConfig {
	result := DefaultFirewallDetection()
	defaults := DefaultFirewallDetection()
	conflicts := map[string][3]bool{}
	banConflict := false
	for index, policy := range policies {
		policy.Enabled = true
		policy.Protect = true
		ban := int(policy.BanDuration().Seconds())
		if index == 0 {
			result.BanSeconds = ban
		} else if result.BanSeconds != ban {
			banConflict = true
			result.BanSeconds = defaults.BanSeconds
			result.MigrationNotice = true
		}
		if banConflict {
			result.BanSeconds = defaults.BanSeconds
		}
		for _, name := range wire.FirewallRuleNames() {
			next, _ := policy.Rule(name)
			if index == 0 {
				result.Rules[name] = next
				continue
			}
			old, base, flags := result.Rules[name], defaults.Rules[name], conflicts[name]
			flags[0] = flags[0] || old.Enabled != next.Enabled
			flags[1] = flags[1] || old.Threshold != next.Threshold
			flags[2] = flags[2] || old.WindowSeconds != next.WindowSeconds
			if flags[0] {
				old.Enabled = base.Enabled
			}
			if flags[1] {
				old.Threshold = base.Threshold
			}
			if flags[2] {
				old.WindowSeconds = base.WindowSeconds
			}
			result.MigrationNotice = result.MigrationNotice || flags[0] || flags[1] || flags[2]
			result.Rules[name] = old
			conflicts[name] = flags
		}
	}
	return result
}
