package model

import "time"

// HourlyPolicy stores administrator terms using explicit payload and TXB units.
type HourlyPolicy struct {
	SalesEnabled              bool   `json:"salesEnabled"`
	DailyAllowanceBytes       int64  `json:"dailyAllowanceBytes,string"`
	ReducedRateBytesPerSecond uint64 `json:"reducedRateBytesPerSecond,string"`
	ExtraGBPriceTXBMinor      int64  `json:"extraGBPriceTXBMinor,string"`
	ActivationHours           int64  `json:"activationHours"`
	CooldownSeconds           int64  `json:"cooldownSeconds"`
}

// DefaultHourlyPolicy leaves sales and unconfigured prices disabled.
func DefaultHourlyPolicy() HourlyPolicy {
	return HourlyPolicy{ActivationHours: 168, CooldownSeconds: 24 * 60 * 60}
}

// HourlyPolicySnapshot binds a validated policy to its encrypted setting CAS.
type HourlyPolicySnapshot struct {
	Version string       `json:"version"`
	Policy  HourlyPolicy `json:"policy"`
}

// HourlyOffer does not change grandfathered fixed-term merchandising.
type HourlyOffer struct {
	GroupID             string    `json:"groupId"`
	Enabled             bool      `json:"enabled"`
	HourlyPriceTXBMinor int64     `json:"-"`
	HourlyPrice         Money     `json:"hourlyPrice"`
	Revision            int64     `json:"revision,string"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}
