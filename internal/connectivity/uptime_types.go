package connectivity

import "time"

// UptimeSegment is a measured or unknown interval, with no probe diagnostics.
type UptimeSegment struct {
	From  time.Time `json:"from"`
	To    time.Time `json:"to"`
	State string    `json:"state"`
}

// UptimeTimeline covers the complete rolling window, including neutral gaps.
type UptimeTimeline struct {
	From     time.Time       `json:"from"`
	To       time.Time       `json:"to"`
	State    string          `json:"state"`
	Segments []UptimeSegment `json:"segments"`
}

// UptimeSummary is the credential-free home projection.
type UptimeSummary struct {
	MonitoringEnabled bool `json:"monitoringEnabled"`
	ActiveCombo       bool `json:"activeCombo"`
	UptimeTimeline
	ErrorCode string `json:"errorCode"`
}

// SubscriptionHost contains only this member's native import link and metadata.
type SubscriptionHost struct {
	UUID          string         `json:"uuid"`
	Name          string         `json:"name"`
	CountryCodes  []string       `json:"countryCodes"`
	SquadUUIDs    []string       `json:"squadUuids"`
	Link          *string        `json:"link"`
	LinkErrorCode string         `json:"linkErrorCode"`
	Timeline      UptimeTimeline `json:"timeline"`
}

// SubscriptionSquad references shared hosts rather than duplicating their links.
type SubscriptionSquad struct {
	UUID      string         `json:"uuid"`
	Name      string         `json:"name"`
	HostUUIDs []string       `json:"hostUuids"`
	Timeline  UptimeTimeline `json:"timeline"`
}

// MemberSubscription separates local eligibility from upstream measurements.
type MemberSubscription struct {
	ActiveCombo     bool                `json:"activeCombo"`
	SubscriptionURL *string             `json:"subscriptionUrl"`
	Summary         UptimeSummary       `json:"summary"`
	Hosts           []SubscriptionHost  `json:"hosts"`
	Squads          []SubscriptionSquad `json:"squads"`
}
