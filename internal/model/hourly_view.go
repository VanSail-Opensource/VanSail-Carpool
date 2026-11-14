package model

import "time"

// HourlyAccountUsage exposes committed accounting, never live-rate authority.
type HourlyAccountUsage struct {
	DayUTC                    string                  `json:"dayUTC"`
	FreeAllowanceBytes        int64                   `json:"freeAllowanceBytes,string"`
	FreeUsedBytes             int64                   `json:"freeUsedBytes,string"`
	FreeReservedBytes         int64                   `json:"freeReservedBytes,string"`
	ExtraFundedBytes          int64                   `json:"extraFundedBytes,string"`
	ExtraUsedBytes            int64                   `json:"extraUsedBytes,string"`
	ExtraReservedBytes        int64                   `json:"extraReservedBytes,string"`
	ExtraExpiresAt            time.Time               `json:"extraExpiresAt"`
	ReducedRateBytesPerSecond uint64                  `json:"reducedRateBytesPerSecond,string"`
	AccountingPending         bool                    `json:"accountingPending"`
	Accounting                *HourlyAccountingStatus `json:"accounting,omitempty"`
}

// HourlyAccountingStatus separates open credit from unconfirmed usage evidence.
// Current amounts belong to DayUTC; historical amounts belong to other UTC
// days and do not reduce the current day's allowance. Exact decimal-string byte
// totals may exceed int64 across pools/days. No field releases credit.
type HourlyAccountingStatus struct {
	OpenReservedBytes               string `json:"openReservedBytes"`
	RecoveryUnknownBytes            string `json:"recoveryUnknownBytes"`
	FreeRecoveryUnknownBytes        string `json:"freeRecoveryUnknownBytes"`
	ExtraRecoveryUnknownBytes       string `json:"extraRecoveryUnknownBytes"`
	HistoricalRecoveryUnknownBytes  string `json:"historicalRecoveryUnknownBytes"`
	HistoricalUnsealedReservedBytes string `json:"historicalUnsealedReservedBytes"`
	HistoricalUnsealedCount         int64  `json:"historicalUnsealedCount,string"`
}

// HourlySquadView overlays one current native squad without mirroring its data.
type HourlySquadView struct {
	Offer           HourlyOffer       `json:"offer"`
	Squad           SquadProduct      `json:"squad"`
	Enrollment      *HourlyEnrollment `json:"enrollment"`
	Ready           bool              `json:"ready"`
	ReasonCode      string            `json:"reasonCode"`
	CooldownUntil   *time.Time        `json:"cooldownUntil"`
	RequiredBalance *Money            `json:"requiredBalance"`
}

// HourlyAccountView is owner-scoped and contains no native credentials.
type HourlyAccountView struct {
	Policy          HourlyPolicySnapshot  `json:"policy"`
	Balance         Money                 `json:"balance"`
	LegacyBlocked   bool                  `json:"legacyBlocked"`
	ExtraPreference HourlyExtraPreference `json:"extraPreference"`
	Usage           *HourlyAccountUsage   `json:"usage"`
	Squads          []HourlySquadView     `json:"squads"`
}
