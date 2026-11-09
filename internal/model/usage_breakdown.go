package model

import "time"

const MaxUsageBreakdownEntries = 128

// MaxUsageBreakdownNodes is retained for callers of the original node contract.
const MaxUsageBreakdownNodes = MaxUsageBreakdownEntries

const (
	UsageLegacyNone                = "none"
	UsageLegacyAvailable           = "available"
	UsageLegacyOriginalTermUnknown = "original_term_unknown"
)

// RationalMinor is an exact nonnegative TXB-minor amount. Both decimal integers
// are reduced by their GCD; denominator is positive and zero is always 0/1.
type RationalMinor struct {
	Numerator   string `json:"numerator"`
	Denominator string `json:"denominator"`
}

// UsageCoverage describes forward recorded history, never invented backfill.
type UsageCoverage struct {
	Since     *time.Time `json:"since"`
	Complete  bool       `json:"complete"`
	GapCount  string     `json:"gapCount"`
	Truncated bool       `json:"truncated"`
}

type UsageBreakdownNode struct {
	NodeID        string        `json:"nodeId"`
	Name          string        `json:"name"`
	CountryCode   string        `json:"countryCode"`
	UploadBytes   string        `json:"uploadBytes"`
	DownloadBytes string        `json:"downloadBytes"`
	CostMinor     RationalMinor `json:"costMinor"`
}

// UsageBreakdownHourlySquad aggregates one named squad across its retained
// enrollments and physical nodes. Costs use recorded settlements only.
type UsageBreakdownHourlySquad struct {
	GroupID       string        `json:"groupId"`
	Name          string        `json:"name"`
	UploadBytes   string        `json:"uploadBytes"`
	DownloadBytes string        `json:"downloadBytes"`
	CostMinor     RationalMinor `json:"costMinor"`
}

// UsageBreakdownTotals also represents omitted identities without inventing a
// physical node or hourly squad. It is folded into Others after entry counting.
type UsageBreakdownTotals struct {
	UploadBytes   string        `json:"uploadBytes"`
	DownloadBytes string        `json:"downloadBytes"`
	CostMinor     RationalMinor `json:"costMinor"`
}

type UsageBreakdownPositiveNodes struct {
	Traffic string `json:"traffic"`
	Cost    string `json:"cost"`
}

type UsageLegacyEstimate struct {
	Status      string         `json:"status"`
	AmountMinor *RationalMinor `json:"amountMinor"`
}

type UsageBreakdownCost struct {
	ActualMinor      string              `json:"actualMinor"`
	Legacy           UsageLegacyEstimate `json:"legacy"`
	KnownTotalMinor  RationalMinor       `json:"knownTotalMinor"`
	UnallocatedMinor RationalMinor       `json:"unallocatedMinor"`
}

// UsageBreakdown contains owner-scoped recorded bytes and exact charge facts.
// Node cost combines recorded traffic charges and allocated legacy estimates;
// hourly settlement costs belong to squad entries; shared extras remain
// unallocated. Principal IDs/generations and
// credentials never enter this presentation contract.
type UsageBreakdown struct {
	From         time.Time                   `json:"from"`
	To           time.Time                   `json:"to"`
	Coverage     UsageCoverage               `json:"coverage"`
	Nodes        []UsageBreakdownNode        `json:"nodes"`
	HourlySquads []UsageBreakdownHourlySquad `json:"hourlySquads"`
	Omitted      UsageBreakdownTotals        `json:"omitted"`
	// PositiveNodes retains physical-only counts for older consumers.
	PositiveNodes UsageBreakdownPositiveNodes `json:"positiveNodes"`
	// PositiveEntries counts physical nodes and hourly squads before the cap.
	PositiveEntries UsageBreakdownPositiveNodes `json:"positiveEntries"`
	Cost            UsageBreakdownCost          `json:"cost"`
}
