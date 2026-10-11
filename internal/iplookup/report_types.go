package iplookup

// Database records a contacted database without retaining its individual result.
type Database struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Refusal retains only the positive item, its origin, and useful evidence.
type Refusal struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Value  any    `json:"value"`
}

// MaxMindFacts preserves requested Insights values, including explicit zeroes.
type MaxMindFacts struct {
	IPRiskSnapshot *float64 `json:"ip_risk_snapshot"`
	StaticIPScore  *float64 `json:"static_ip_score"`
	UserCount      *int     `json:"user_count"`
	UserType       *string  `json:"user_type"`
}

// Stage is a minimal durable marker; it never contains provider response data.
type Stage struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Attempted bool   `json:"attempted"`
	// Complete freezes the provider's own completeness flag so background
	// repair can recompute aggregate completeness exactly.
	Complete bool `json:"complete,omitempty"`
}

// GeoCandidate is one coherent location and its application comparison metadata.
type GeoCandidate struct {
	Country   string   `json:"country"`
	City      string   `json:"city"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Source    string   `json:"source"`
	Provider  string   `json:"provider"`
	Band      int      `json:"band"`
	RadiusKM  *float64 `json:"radiusKm"`
}

// Checkpoint stores aggregate recovery state, never one result per provider.
type Checkpoint struct {
	Stages   []Stage             `json:"stages"`
	Coverage uint8               `json:"coverage"`
	Complete bool                `json:"complete"`
	Usable   int                 `json:"usable"`
	Votes    map[string][]string `json:"votes"`
	Geo      *GeoCandidate       `json:"geo"`
	RPKI     *Stage              `json:"rpki,omitempty"`
	Enhanced *EnhancedCheckpoint `json:"enhanced,omitempty"`
}
