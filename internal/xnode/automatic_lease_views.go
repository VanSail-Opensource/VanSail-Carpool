package xnode

import "time"

type AutomaticLeaseView struct {
	IP                   string     `json:"ip"`
	FirstActive          time.Time  `json:"firstActive"`
	LastActive           time.Time  `json:"lastActive"`
	LeaseStarted         time.Time  `json:"leaseStarted"`
	RenewAfter           time.Time  `json:"renewAfter"`
	RenewedAt            *time.Time `json:"renewedAt"`
	ExpiresAt            time.Time  `json:"expiresAt"`
	RemainingSeconds     int64      `json:"remainingSeconds"`
	ObservationCount     uint64     `json:"observationCount,string"`
	RenewalCount         uint64     `json:"renewalCount,string"`
	SourceCount          int        `json:"sourceCount"`
	SourceCountTruncated bool       `json:"sourceCountTruncated"`
	SourceNodeIDs        []string   `json:"sourceNodeIds"`
	PendingNodes         int        `json:"pendingNodes"`
	BanReleasePending    bool       `json:"banReleasePending"`
}
type AutomaticLeaseStatistics struct {
	Active   int    `json:"active"`
	IPv4     int    `json:"ipv4"`
	IPv6     int    `json:"ipv6"`
	Created  uint64 `json:"created,string"`
	Renewed  uint64 `json:"renewed,string"`
	Expired  uint64 `json:"expired,string"`
	Rejected uint64 `json:"rejected,string"`
}
type AutomaticLeasePage struct {
	Items      []AutomaticLeaseView     `json:"items"`
	Total      int                      `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"pageSize"`
	Statistics AutomaticLeaseStatistics `json:"statistics"`
	Settings   AutomaticWhitelistConfig `json:"settings"`
	FetchedAt  time.Time                `json:"fetchedAt"`
}
