package iplookup

// ProxySource configures an administrator-named public endpoint inventory.
type ProxySource struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}
type ProxySourceResult struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	ObservedAt string `json:"observedAt"`
	Count      int    `json:"count"`
}
type ProxyAssessment struct {
	Status  string              `json:"status"`
	Sources []ProxySourceResult `json:"sources"`
	Matches []string            `json:"matches"`
}
type SubnetAddress struct {
	IP      string   `json:"ip"`
	Status  string   `json:"status"`
	Proxy   string   `json:"proxy"`
	Abuse   string   `json:"abuse"`
	IPAPI   string   `json:"ipapi"`
	Sources []string `json:"sources"`
}
type SubnetResult struct {
	Prefix    string          `json:"prefix"`
	Status    string          `json:"status"`
	Reason    string          `json:"reason"`
	Addresses []SubnetAddress `json:"addresses"`
}
type ProbeFinding struct {
	Protocol   string `json:"protocol"`
	Confidence string `json:"confidence"`
	Evidence   string `json:"evidence"`
}
type PortProbe struct {
	Port     int            `json:"port"`
	Status   string         `json:"status"`
	Tasks    []ProbeTask    `json:"tasks"`
	Findings []ProbeFinding `json:"findings"`
}
type ProbeTask struct {
	Key           string `json:"key"`
	Status        string `json:"status"`
	StartedAt     string `json:"startedAt"`
	NextMilestone int    `json:"nextMilestone"`
	ErrorCode     string `json:"errorCode"`
}
type EnhancedCheckpoint struct {
	Phase    string          `json:"phase"`
	LiveDone bool            `json:"liveDone"`
	IPAPI    *ProviderResult `json:"ipapi,omitempty"`
	Scores   *NetworkScores  `json:"scores,omitempty"`
}

// SharedProgress contains no ownership, billing, credentials, or private checkpoints.
type SharedProgress struct {
	IP        string        `json:"ip"`
	Status    string        `json:"status"`
	Progress  *Progress     `json:"progress"`
	Ports     []PortProbe   `json:"ports"`
	Subnet    *SubnetResult `json:"subnet"`
	UpdatedAt string        `json:"updatedAt"`
}
