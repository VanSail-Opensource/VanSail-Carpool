package httpapi

import "github.com/VanSail-Opensource/VanSail-Carpool/internal/model"

// Presence is part of a complete policy replacement. Missing/null fields must
// not silently clear prices, disable offers, or remove the reviewed cooldown.
type adminHourlyPolicyInput struct {
	Version *string `json:"version"`
	Policy  *struct {
		SalesEnabled              *bool   `json:"salesEnabled"`
		DailyAllowanceBytes       *int64  `json:"dailyAllowanceBytes,string"`
		ReducedRateBytesPerSecond *uint64 `json:"reducedRateBytesPerSecond,string"`
		ExtraGBPriceTXBMinor      *int64  `json:"extraGBPriceTXBMinor,string"`
		ActivationHours           *int64  `json:"activationHours"`
		CooldownSeconds           *int64  `json:"cooldownSeconds"`
	} `json:"policy"`
}

func (input adminHourlyPolicyInput) snapshot() (model.HourlyPolicySnapshot, bool) {
	p := input.Policy
	if input.Version == nil || p == nil || p.SalesEnabled == nil || p.DailyAllowanceBytes == nil ||
		p.ReducedRateBytesPerSecond == nil || p.ExtraGBPriceTXBMinor == nil || p.ActivationHours == nil || p.CooldownSeconds == nil {
		return model.HourlyPolicySnapshot{}, false
	}
	return model.HourlyPolicySnapshot{Version: *input.Version, Policy: model.HourlyPolicy{
		SalesEnabled: *p.SalesEnabled, DailyAllowanceBytes: *p.DailyAllowanceBytes,
		ReducedRateBytesPerSecond: *p.ReducedRateBytesPerSecond, ExtraGBPriceTXBMinor: *p.ExtraGBPriceTXBMinor,
		ActivationHours: *p.ActivationHours, CooldownSeconds: *p.CooldownSeconds,
	}}, true
}

type adminHourlyOfferInput struct {
	Revision            *int64 `json:"revision,string"`
	Enabled             *bool  `json:"enabled"`
	HourlyPriceTXBMinor *int64 `json:"hourlyPriceTXBMinor,string"`
	RequiredHours       *int64 `json:"requiredHours"`
}

func (input adminHourlyOfferInput) complete() bool {
	return input.Revision != nil && input.Enabled != nil && input.HourlyPriceTXBMinor != nil && input.RequiredHours != nil
}
