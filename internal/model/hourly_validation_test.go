package model

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestHourlyPolicyDefaultsAndExplicitUnitBounds(t *testing.T) {
	defaults := DefaultHourlyPolicy()
	if defaults.SalesEnabled || defaults.ActivationHours != 168 || defaults.CooldownSeconds != 86400 || defaults.DailyAllowanceBytes != 0 || defaults.ExtraGBPriceTXBMinor != 0 || ValidateHourlyPolicy(defaults) != nil {
		t.Fatal("unconfigured defaults enabled a sale or changed contract defaults")
	}
	for _, test := range []struct {
		name   string
		change func(*HourlyPolicy)
		valid  bool
	}{
		{"zero activation", func(p *HourlyPolicy) { p.ActivationHours = 0 }, false},
		{"negative allowance", func(p *HourlyPolicy) { p.DailyAllowanceBytes = -1 }, false},
		{"zero allowance", func(p *HourlyPolicy) { p.DailyAllowanceBytes = 0 }, true},
		{"zero enabled rate", func(p *HourlyPolicy) { p.SalesEnabled = true }, false},
		{"maximum enabled rate", func(p *HourlyPolicy) { p.SalesEnabled, p.ReducedRateBytesPerSecond = true, 1_000_000_000_000 }, true},
		{"excess rate", func(p *HourlyPolicy) { p.ReducedRateBytesPerSecond = 1_000_000_000_001 }, false},
		{"unavailable extras", func(p *HourlyPolicy) { p.ExtraGBPriceTXBMinor = 0 }, true},
		{"negative extra price", func(p *HourlyPolicy) { p.ExtraGBPriceTXBMinor = -1 }, false},
		{"zero cooldown", func(p *HourlyPolicy) { p.CooldownSeconds = 0 }, true},
		{"negative cooldown", func(p *HourlyPolicy) { p.CooldownSeconds = -1 }, false},
		{"cooldown duration overflow", func(p *HourlyPolicy) { p.CooldownSeconds = int64(time.Duration(math.MaxInt64)/time.Second) + 1 }, false},
		{"activation duration overflow", func(p *HourlyPolicy) { p.ActivationHours = int64(time.Duration(math.MaxInt64)/time.Hour) + 1 }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			policy := defaults
			test.change(&policy)
			err := ValidateHourlyPolicy(policy)
			if (err == nil) != test.valid || !test.valid && !errors.Is(err, ErrHourlyPolicyInvalid) {
				t.Fatal("unexpected unit validation", err)
			}
		})
	}
}

func TestHourlyActivationEligibilityPreservesExactMoneyBounds(t *testing.T) {
	policy := DefaultHourlyPolicy()
	price := int64(math.MaxInt64 / 168)
	minimum, err := HourlyActivationMinimum(policy, 0, price)
	if err != nil || minimum != price*168 {
		t.Fatal("maximum safe threshold lost exact minor units", minimum, err)
	}
	for _, invalid := range []int64{0, -1, price + 1, math.MaxInt64} {
		if amount, err := HourlyActivationMinimum(policy, 0, invalid); !errors.Is(err, ErrHourlyPolicyInvalid) || amount != 0 {
			t.Fatal("invalid or overflowing threshold accepted", invalid, amount, err)
		}
	}
	policy.ActivationHours = 1
	if amount, err := HourlyActivationMinimum(policy, 0, math.MaxInt64); err != nil || amount != math.MaxInt64 {
		t.Fatal("exact int64 maximum was rejected", amount, err)
	}
	policy.ActivationHours = 0
	if _, err := HourlyActivationMinimum(policy, 0, 1); !errors.Is(err, ErrHourlyPolicyInvalid) {
		t.Fatal("invalid policy bypassed threshold validation", err)
	}
}

func TestHourlyActivationUsesPerOfferRequiredHoursOrDefault(t *testing.T) {
	policy := DefaultHourlyPolicy()
	if amount, err := HourlyActivationMinimum(policy, 0, 101); err != nil || amount != 101*168 {
		t.Fatal("zero required hours did not inherit the policy threshold", amount, err)
	}
	if amount, err := HourlyActivationMinimum(policy, 2, 101); err != nil || amount != 202 {
		t.Fatal("per-offer required hours were ignored", amount, err)
	}
	if amount, err := HourlyActivationMinimum(policy, MaxHourlyRequiredHours, 1); err != nil || amount != MaxHourlyRequiredHours {
		t.Fatal("maximum per-offer required hours were rejected", amount, err)
	}
	for _, invalid := range []int64{-1, MaxHourlyRequiredHours + 1, math.MaxInt64} {
		if amount, err := HourlyActivationMinimum(policy, invalid, 101); !errors.Is(err, ErrHourlyPolicyInvalid) || amount != 0 {
			t.Fatal("invalid per-offer required hours accepted", invalid, amount, err)
		}
	}
	if _, err := HourlyActivationMinimum(policy, 2, math.MaxInt64); !errors.Is(err, ErrHourlyPolicyInvalid) {
		t.Fatal("per-offer threshold overflow was accepted")
	}
}

func TestHourlyPolicyLargeUnitsUseDecimalStrings(t *testing.T) {
	policy := DefaultHourlyPolicy()
	policy.DailyAllowanceBytes, policy.ExtraGBPriceTXBMinor = 1<<60+7, 1<<60+9
	policy.ReducedRateBytesPerSecond = 1_000_000_000_000
	raw, err := json.Marshal(policy)
	if err != nil || !strings.Contains(string(raw), `"dailyAllowanceBytes":"1152921504606846983"`) || !strings.Contains(string(raw), `"extraGBPriceTXBMinor":"1152921504606846985"`) || !strings.Contains(string(raw), `"reducedRateBytesPerSecond":"1000000000000"`) {
		t.Fatal("policy units lost decimal-string serialization", err)
	}
	var restored HourlyPolicy
	if err := json.Unmarshal(raw, &restored); err != nil || restored != policy {
		t.Fatal("policy units did not round trip exactly", err)
	}
	if err := json.Unmarshal([]byte(`{"dailyAllowanceBytes":1152921504606846983}`), &restored); err == nil {
		t.Fatal("unsafe JSON number accepted for a decimal-string byte count")
	}
}
