package iplookup

// ProxySource configures an administrator-named public endpoint inventory.
type ProxySource struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	URL      string              `json:"url"`
	Enabled  bool                `json:"enabled"`
	Category ProxySourceCategory `json:"category"`
}
type ProxySourceResult struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Category   ProxySourceCategory `json:"category"`
	Status     string              `json:"status"`
	ObservedAt string              `json:"observedAt"`
	Count      int                 `json:"count"`
	URL        string              `json:"url,omitempty"`
	Phase      string              `json:"phase,omitempty"`
	Hosts      int                 `json:"hosts,omitempty"`
	Processed  int                 `json:"processed,omitempty"`
	Skipped    int                 `json:"skipped,omitempty"`
	DNSFailed  int                 `json:"dnsFailed,omitempty"`
	ErrorCode  string              `json:"errorCode,omitempty"`
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
	Prefix    string            `json:"prefix"`
	Status    string            `json:"status"`
	Reason    string            `json:"reason"`
	Addresses []SubnetAddress   `json:"addresses"`
	Sources   []InventorySource `json:"sources,omitempty"`
}
type ProbeFinding struct {
	Protocol   string    `json:"protocol"`
	Confidence string    `json:"confidence"`
	Evidence   string    `json:"evidence"`
	TaskKey    string    `json:"taskKey,omitempty"`
	DNS        *ProbeDNS `json:"dns,omitempty"`
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
	Result        string `json:"result,omitempty"`
}

// ProbeDNS retains bounded DNS evidence without subscription payloads.
type ProbeDNS struct {
	Domain    string           `json:"domain"`
	Resolvers []DNSObservation `json:"resolvers"`
}

// DNSObservation describes the response from one resolver.
type DNSObservation struct {
	Server    string   `json:"server"`
	Status    string   `json:"status"`
	Addresses []string `json:"addresses"`
}

// ProxySourceTest is an ephemeral administrator diagnostic.
type ProxySourceTest struct {
	ID     string            `json:"id"`
	Status string            `json:"status"`
	Result ProxySourceResult `json:"result"`
}
type EnhancedCheckpoint struct {
	InventoryDone bool            `json:"inventoryDone,omitempty"`
	Phase         string          `json:"phase"`
	LiveDone      bool            `json:"liveDone"`
	IPAPI         *ProviderResult `json:"ipapi,omitempty"`
	Scores        *NetworkScores  `json:"scores,omitempty"`
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
