package renewal

// Candidate retains the public renewal price and immutable historical trace fields.
// Individual-response recommendation code is retired; the database adapter
// selects the current public price within the purchased contractual bounds.
type Candidate struct {
	Price        int64   `json:"priceMinor,string"`
	Probability  float64 `json:"probability"`
	Contribution float64 `json:"contribution"`
	Utility      float64 `json:"utility"`
	Eligible     bool    `json:"eligible"`
}
