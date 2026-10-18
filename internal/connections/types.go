// Package connections owns member connection scans and signed drop handles.
package connections

import (
	"errors"
	"time"
)

const HandleTTL = 15 * time.Minute

// BlockDuration is the fixed member-requested IP block window.
const BlockDuration = 72 * time.Hour

const (
	// ScanTTL bounds provider job metadata and all handles derived from it.
	ScanTTL = 30 * time.Minute
	// ScanRequestOutboxKind starts a metadata-only provider connection scan.
	ScanRequestOutboxKind = "connection_scan_request"
	// DropOperationKind owns durable selected-IP unlink operations.
	DropOperationKind = "connection_drop"
	// BlockOperationKind blocks one selected IP before disconnecting it.
	BlockOperationKind = "connection_block"
	// UnblockOperationKind removes one active node-scoped IP block.
	UnblockOperationKind = "connection_unblock"
	// BlockExpiryOutboxKind is the scheduled unblock backstop.
	BlockExpiryOutboxKind = "connection_ip_block_expiry"
)

const (
	BlockStatusBlocking      = "blocking"
	BlockStatusActive        = "active"
	BlockStatusUnblocking    = "unblocking"
	BlockStatusPendingReview = "pending_review"
)

var (
	// ErrScanNotFound hides scans owned by another member and expired scans.
	ErrScanNotFound = errors.New("connection scan not found")
	// ErrIdentityRequired means the member has no usable provider identity.
	ErrIdentityRequired = errors.New("Remnawave identity is required")
	// ErrIPBlockNotFound hides blocks owned by another member.
	ErrIPBlockNotFound = errors.New("connection IP block not found")
)

// HandleClaims are the server-authoritative fields carried by a signed handle.
type HandleClaims struct {
	UserID   string
	ScanID   string
	NodeUUID string
	IP       string
	Expires  time.Time
}

// IP is one display-safe connection observation with its drop capability.
type IP struct {
	Address  string    `json:"ip"`
	LastSeen time.Time `json:"lastSeen"`
	Handle   string    `json:"handle"`
}

// Node groups current member connections by provider node.
type Node struct {
	Unavailable bool   `json:"unavailable"`
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	CountryCode string `json:"countryCode"`
	IPs         []IP   `json:"ips"`
}

// Scan is the transient browser projection; only metadata is durable.
type Scan struct {
	ID              string    `json:"id"`
	Completed       bool      `json:"isCompleted"`
	Failed          bool      `json:"isFailed"`
	ProgressPercent float64   `json:"progressPercent"`
	Nodes           []Node    `json:"nodes"`
	CreatedAt       time.Time `json:"createdAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

// Observation is one provider result before a short-lived handle is attached.
type Observation struct {
	Address  string
	LastSeen time.Time
}

// ProviderNode is a provider scan node before browser projection.
type ProviderNode struct {
	Unavailable bool
	UUID        string
	Name        string
	CountryCode string
	IPs         []Observation
}

// ProviderScan is the normalized, non-durable Remnawave scan response.
type ProviderScan struct {
	Completed       bool
	Failed          bool
	ProgressPercent float64
	Nodes           []ProviderNode
}

// IPBlock is the member-safe projection of one active encrypted block.
type IPBlock struct {
	ID        string    `json:"id"`
	IP        string    `json:"ip"`
	NodeUUID  string    `json:"nodeUuid"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// IPBlockRecord is the internal encrypted active-row representation.
type IPBlockRecord struct {
	ID                 string
	UserID             string
	NodeUUID           string
	IPDigest           string
	SealedIP           string
	Status             string
	BlockOperationID   string
	UnblockOperationID string
	ExpiryJobID        string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ExpiresAt          time.Time
}

// CreateIPBlockInput contains the sensitive data already sealed by the service.
type CreateIPBlockInput struct {
	UserID    string
	NodeUUID  string
	IPDigest  string
	SealedIP  string
	ExpiresAt time.Time
}
