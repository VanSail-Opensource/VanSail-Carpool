package xnode

import "time"

// MemberConnection contains no node identity or cross-owner observation data.
type MemberConnection struct {
	IP             string     `json:"ip"`
	InitialLeaseAt *time.Time `json:"initialLeaseAt"`
	LastObservedAt *time.Time `json:"lastObservedAt"`
	ExpiresAt      *time.Time `json:"expiresAt"`
}

type MemberConnectionPage struct {
	Items      []MemberConnection       `json:"items"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"pageSize"`
	Total      int                      `json:"total"`
	Status     string                   `json:"status"`
	Incomplete bool                     `json:"incomplete"`
	CanGrant   bool                     `json:"canGrant"`
	Settings   AutomaticWhitelistConfig `json:"settings"`
	FetchedAt  time.Time                `json:"fetchedAt"`
}
