package xnode

import (
	"encoding/json"
	"time"
)

type Credentials struct {
	UUID        string                     `json:"uuid"`
	Trojan      string                     `json:"trojan"`
	Shadowsocks string                     `json:"shadowsocks"`
	SS2022_128  string                     `json:"ss2022_128"`
	SS2022_256  string                     `json:"ss2022_256"`
	Inbound     map[string]json.RawMessage `json:"inbound,omitempty"`
}

type Identity struct {
	ProxyUserID        int64     `json:"proxyUserId,string"`
	UserID             string    `json:"userId"`
	Username           string    `json:"username"`
	ServiceName        string    `json:"serviceName"`
	CredentialsCipher  string    `json:"-"`
	SubscriptionHash   []byte    `json:"-"`
	SubscriptionCipher string    `json:"-"`
	CurrentPeriodID    string    `json:"currentPeriodId"`
	LegacyUserID       string    `json:"-"`
	LifetimeBase       int64     `json:"-"`
	AccessDisabled     bool      `json:"accessDisabled"`
	CreatedAt          time.Time `json:"createdAt"`
}

type Period struct {
	ID          string     `json:"id"`
	ProxyUserID int64      `json:"proxyUserId,string"`
	PurchaseID  string     `json:"purchaseId"`
	StartedAt   time.Time  `json:"startedAt"`
	EndedAt     *time.Time `json:"endedAt"`
	LimitBytes  int64      `json:"limitBytes,string"`
	Strategy    string     `json:"strategy"`
	LastResetAt *time.Time `json:"lastResetAt"`
	UsedBytes   int64      `json:"usedBytes,string"`
	Status      string     `json:"status"`
}

type DailyUsage struct {
	ProxyUserID   int64
	NodeID        string
	PeriodID      string
	Date          string
	UploadBytes   int64
	DownloadBytes int64
	WeightedBytes int64
}

// LegacyHistory is a cumulative daily source export used for history only.
// Quota and lifetime come from the already weighted aggregate, never this series.
type LegacyHistory struct {
	ProxyUserID   int64
	NodeID        string
	Date          string
	RawBytes      int64
	WeightedBytes int64
}
type LegacyUsage struct {
	ProxyUserID   int64
	PeriodID      string
	UsedBytes     int64
	LifetimeBytes int64
	LastResetAt   *time.Time
	SyncedAt      time.Time
}
type PeriodNode struct {
	NodeID   string
	Flushed  bool
	Sequence uint64
}
type ServiceBinding struct {
	ProxyUserID int64
	NodeID      string
	InboundTag  string
	ValidUntil  time.Time
}

type Customization struct {
	UserID          string          `json:"-"`
	NodeID          string          `json:"nodeId"`
	OutboundsCipher string          `json:"-"`
	RoutingCipher   string          `json:"-"`
	Outbounds       json.RawMessage `json:"outbounds"`
	Routing         json.RawMessage `json:"routing"`
	Revision        int64           `json:"revision"`
}

type MigrationState struct {
	Phase                 string          `json:"phase"`
	SourceCipher          string          `json:"-"`
	SnapshotCipher        string          `json:"-"`
	SnapshotDigest        string          `json:"snapshotDigest"`
	SourceBackupReference string          `json:"sourceBackupReference"`
	BackupName            string          `json:"backupName"`
	Nodes                 []MigrationNode `json:"nodes"`
}
type MigrationNode struct {
	NodeID       string     `json:"nodeId"`
	LegacyNodeID string     `json:"legacyNodeId"`
	Phase        string     `json:"phase"`
	SourceDigest string     `json:"sourceDigest"`
	UsageCutoff  *time.Time `json:"usageCutoff"`
	ErrorCode    string     `json:"errorCode"`
}
