package iplookup

// CloudflareID identifies the optional enrichment credential, not a risk provider.
const CloudflareID = "cloudflare_radar"

// QueueIDs includes the existing reputation queues and live enrichment sources.
func QueueIDs() []string {
	return append(append([]string{}, IDs...), "ripe", "routeviews", "caida", "bgpkit", CloudflareID, "dns", "shodan", "peeringdb", "checks", "proxy_lists", "probes")
}

// LiveMeta describes one source's availability and observation time.
type LiveMeta struct {
	Status     string `json:"status"`
	Source     string `json:"source"`
	ObservedAt string `json:"observedAt"`
	ErrorCode  string `json:"errorCode"`
}

// LiveDetails is response-only information for the requested address.
type LiveDetails struct {
	SnapshotID string           `json:"-"`
	SourceIP   string           `json:"sourceIP"`
	CacheMatch string           `json:"cacheMatch"`
	IP         string           `json:"ip"`
	FetchedAt  string           `json:"fetchedAt"`
	Topology   BGPTopology      `json:"topology"`
	ASNs       []ASNDetails     `json:"asns"`
	Scores     NetworkScores    `json:"scores"`
	Block      AbuseBlock       `json:"block"`
	PTR        PTRDetails       `json:"ptr"`
	Shodan     ShodanDetails    `json:"shodan"`
	Transfers  *TransferHistory `json:"transfers,omitempty"`
}

// BGPTopology preserves observed paths separately from inferred relationships.
type BGPTopology struct {
	LiveMeta
	Prefix             string            `json:"prefix"`
	Origins            []uint32          `json:"origins"`
	Nodes              []BGPNode         `json:"nodes"`
	Edges              []BGPEdge         `json:"edges"`
	Paths              []BGPPath         `json:"paths"`
	Tier1Links         []Tier1Link       `json:"tier1Links"`
	Truncated          bool              `json:"truncated"`
	RelationshipMeta   LiveMeta          `json:"relationshipMeta"`
	ClassificationMeta LiveMeta          `json:"classificationMeta"`
	ProjectionVersion  int               `json:"projectionVersion,omitempty"`
	RouteSummaries     []BGPRouteSummary `json:"routeSummaries,omitempty"`
}

// Tier1Link summarizes an observed path, never an inferred adjacent AS pair.
type Tier1Link struct {
	Origin   uint32 `json:"origin"`
	Neighbor uint32 `json:"neighbor"`
	Tier1    uint32 `json:"tier1"`
	Hops     int    `json:"hops"`
}

type BGPNode struct {
	ASN            uint32 `json:"asn"`
	Name           string `json:"name"`
	Tier1          *bool  `json:"tier1"`
	ObservedRoutes *int   `json:"observedRoutes,omitempty"`
}

type BGPEdge struct {
	From             uint32    `json:"from"`
	To               uint32    `json:"to"`
	Relationship     string    `json:"relationship"`
	ObservedRoutes   *int      `json:"observedRoutes,omitempty"`
	RelationshipMeta *LiveMeta `json:"relationshipMeta,omitempty"`
}

// BGPRouteSummary contains visible hops only; Hops records each segment's real length.
type BGPRouteSummary struct {
	ASNs           []uint32 `json:"asns"`
	Hops           []int    `json:"hops"`
	ObservedRoutes int      `json:"observedRoutes"`
}

type BGPPath struct {
	ASPath     []uint32 `json:"asPath"`
	Collector  string   `json:"collector"`
	Exchange   string   `json:"exchange"`
	ObservedAt string   `json:"observedAt"`
}

// ASNDetails contains contextual metrics; none participates in suitability policy.
type ASNDetails struct {
	ASN               uint32              `json:"asn"`
	Name              string              `json:"name"`
	RegisteredAt      string              `json:"registeredAt"`
	RIR               string              `json:"rir"`
	Announced         *bool               `json:"announced"`
	IPv4Prefixes      *int                `json:"ipv4Prefixes"`
	IPv6Prefixes      *int                `json:"ipv6Prefixes"`
	ObservedNeighbors *int                `json:"observedNeighbors"`
	Peers             *int                `json:"peers"`
	Upstreams         *int                `json:"upstreams"`
	Sources           map[string]LiveMeta `json:"sources"`
	Traffic           ASNTraffic          `json:"traffic"`
	AbuseContacts     []string            `json:"abuseContacts,omitempty"`
	Website           string              `json:"website,omitempty"`
}

type TrafficSummary struct {
	LiveMeta
	Values    map[string]float64 `json:"values"`
	StartTime string             `json:"startTime"`
	EndTime   string             `json:"endTime"`
}

type ASNTraffic struct {
	BotHuman  TrafficSummary `json:"botHuman"`
	Devices   TrafficSummary `json:"devices"`
	IPVersion TrafficSummary `json:"ipVersion"`
}

type NetworkScore struct {
	Ratio *float64 `json:"ratio"`
	Label string   `json:"label"`
}

type NetworkScores struct {
	LiveMeta
	Company NetworkScore `json:"company"`
	ASN     NetworkScore `json:"asn"`
}

type AbuseBlock struct {
	MinAddress string         `json:"-"`
	MaxAddress string         `json:"-"`
	Addresses  []AbuseAddress `json:"addresses,omitempty"`
	LiveMeta
	Prefix            string `json:"prefix"`
	ReportedAddresses *int   `json:"reportedAddresses"`
	AddressCapacity   string `json:"addressCapacity"`
	WindowDays        int    `json:"windowDays"`
}

// AbuseAddress is compact block evidence without comments or reporter identities.
type AbuseAddress struct {
	IP      string  `json:"ip"`
	Reports int     `json:"reports"`
	Score   float64 `json:"score"`
}

type PTRDetails struct {
	LiveMeta
	Domains []string `json:"domains"`
}

// ShodanDetails contains InternetDB observations, never an active port scan.
type ShodanDetails struct {
	LiveMeta
	Hostnames []string `json:"hostnames"`
	Ports     []int    `json:"ports"`
	CPEs      []string `json:"cpes"`
	Tags      []string `json:"tags"`
	Vulns     []string `json:"vulns"`
}
