package remnawave

import "time"

type InternalSquad struct {
	UUID         string        `json:"uuid"`
	ViewPosition int64         `json:"viewPosition"`
	Name         string        `json:"name"`
	Inbounds     []NodeInbound `json:"inbounds"`
	Info         SquadInfo     `json:"info"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

// NodeInbound is one currently active inbound exposed by a Remnawave node.

type NodeInbound struct {
	UUID        string `json:"uuid"`
	Tag         string `json:"tag"`
	ProfileUUID string `json:"profileUuid"`
}

// NodeProvider is the display-safe provider identity returned with a live node.
type NodeProvider struct {
	Name        string  `json:"name"`
	FaviconLink *string `json:"faviconLink"`
}

// Node is the documented subset required to map node selections to squad
// inbounds. Links and other volatile upstream properties are intentionally not
// represented or persisted.

type Node struct {
	UUID                  string        `json:"uuid"`
	Name                  string        `json:"name"`
	CountryCode           string        `json:"countryCode"`
	ConsumptionMultiplier float64       `json:"consumptionMultiplier"`
	IsDisabled            bool          `json:"isDisabled"`
	IsConnected           bool          `json:"isConnected"`
	UsersOnline           float64       `json:"usersOnline"`
	System                *NodeSystem   `json:"system"`
	Versions              *NodeVersions `json:"versions"`
	Provider              *NodeProvider `json:"provider"`
	ConfigProfile         struct {
		UUID           *string       `json:"activeConfigProfileUuid"`
		ActiveInbounds []NodeInbound `json:"activeInbounds"`
	} `json:"configProfile"`
}

// NodeSystem is the live network-rate subset returned by Get nodes.
type NodeSystem struct {
	Stats struct {
		Interface *struct {
			RXBytesPerSecond float64 `json:"rxBytesPerSec"`
			TXBytesPerSecond float64 `json:"txBytesPerSec"`
		} `json:"interface"`
	} `json:"stats"`
}

// NodeVersions contains the node and Xray versions exposed by Remnawave.
type NodeVersions struct {
	Xray string `json:"xray"`
	Node string `json:"node"`
}

// AccessibleNode is returned after Remnawave resolves squad inbounds to nodes.

type AccessibleNode struct {
	UUID           string   `json:"uuid"`
	NodeName       string   `json:"nodeName"`
	CountryCode    string   `json:"countryCode"`
	ActiveInbounds []string `json:"activeInbounds"`
}

// ValidationIssue describes one Remnawave request validation failure.

type ValidationIssue struct {
	Validation string   `json:"validation"`
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	Path       []string `json:"path"`
}
