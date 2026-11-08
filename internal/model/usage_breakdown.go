package model

import "time"

const MaxUsageBreakdownNodes = 128

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

// UsageBreakdownTotals also represents omitted identities without inventing a
// physical node. It is folded into the UI's Others group after node counting.
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
// hourly/extra charges remain explicitly unallocated. Principal IDs/generations and
// credentials never enter this presentation contract.
type UsageBreakdown struct {
	From          time.Time                   `json:"from"`
	To            time.Time                   `json:"to"`
	Coverage      UsageCoverage               `json:"coverage"`
	Nodes         []UsageBreakdownNode        `json:"nodes"`
	Omitted       UsageBreakdownTotals        `json:"omitted"`
	PositiveNodes UsageBreakdownPositiveNodes `json:"positiveNodes"`
	Cost          UsageBreakdownCost          `json:"cost"`
}
