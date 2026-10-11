package iplookup

// CloudflareID identifies the optional enrichment credential, not a risk provider.
const CloudflareID = "cloudflare_radar"

// QueueIDs includes the existing reputation queues and live enrichment sources.
func QueueIDs() []string {
	return append(append([]string{}, IDs...), "ripe", "routeviews", "caida", "bgpkit", CloudflareID, "dns", "shodan")
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
	SnapshotID string        `json:"-"`
	SourceIP   string        `json:"sourceIP"`
	CacheMatch string        `json:"cacheMatch"`
	IP         string        `json:"ip"`
	FetchedAt  string        `json:"fetchedAt"`
	Topology   BGPTopology   `json:"topology"`
	ASNs       []ASNDetails  `json:"asns"`
	Scores     NetworkScores `json:"scores"`
	Block      AbuseBlock    `json:"block"`
	PTR        PTRDetails    `json:"ptr"`
	Shodan     ShodanDetails `json:"shodan"`
}

// BGPTopology preserves observed paths separately from inferred relationships.
type BGPTopology struct {
	LiveMeta
	Prefix             string      `json:"prefix"`
	Origins            []uint32    `json:"origins"`
	Nodes              []BGPNode   `json:"nodes"`
	Edges              []BGPEdge   `json:"edges"`
	Paths              []BGPPath   `json:"paths"`
	Tier1Links         []Tier1Link `json:"tier1Links"`
	Truncated          bool        `json:"truncated"`
	RelationshipMeta   LiveMeta    `json:"relationshipMeta"`
	ClassificationMeta LiveMeta    `json:"classificationMeta"`
}

// Tier1Link summarizes an observed path, never an inferred adjacent AS pair.
type Tier1Link struct {
	Origin   uint32 `json:"origin"`
	Neighbor uint32 `json:"neighbor"`
	Tier1    uint32 `json:"tier1"`
	Hops     int    `json:"hops"`
}

type BGPNode struct {
	ASN   uint32 `json:"asn"`
	Name  string `json:"name"`
	Tier1 *bool  `json:"tier1"`
}

type BGPEdge struct {
	From         uint32 `json:"from"`
	To           uint32 `json:"to"`
	Relationship string `json:"relationship"`
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
	LiveMeta
	Prefix            string `json:"prefix"`
	ReportedAddresses *int   `json:"reportedAddresses"`
	AddressCapacity   string `json:"addressCapacity"`
	WindowDays        int    `json:"windowDays"`
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
}
