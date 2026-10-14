package xnode

import (
	"encoding/json"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type NodeMetrics struct {
	NodeID                 string            `json:"nodeId"`
	Connected              bool              `json:"connected"`
	Ready                  bool              `json:"ready"`
	Synchronized           bool              `json:"synchronized"`
	Stale                  bool              `json:"stale"`
	Version                string            `json:"version"`
	XrayVersion            string            `json:"xrayVersion"`
	Capabilities           []string          `json:"capabilities"`
	ReportedAt             *time.Time        `json:"reportedAt"`
	Machine                json.RawMessage   `json:"machine"`
	Certificates           json.RawMessage   `json:"certificates"`
	ActiveConnections      int64             `json:"activeConnections"`
	UsersOnline            int64             `json:"usersOnline"`
	UploadBytesPerSecond   float64           `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond float64           `json:"downloadBytesPerSecond"`
	Logs                   []json.RawMessage `json:"logs"`
}

type UserMetrics struct {
	Connections            int64       `json:"connections"`
	UploadBytesPerSecond   float64     `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond float64     `json:"downloadBytesPerSecond"`
	Incomplete             bool        `json:"incomplete"`
	Limited                bool        `json:"limited"`
	Reasons                []UserLimit `json:"reasons"`
	FetchedAt              time.Time   `json:"fetchedAt"`
}
type UserLimit struct {
	NodeID string `json:"nodeId"`
	Reason string `json:"reason"`
	Scope  string `json:"scope"`
}
type MetricsSummary struct {
	Nodes                  []NodeMetrics `json:"nodes"`
	OnlineNodes            int           `json:"onlineNodes"`
	TotalNodes             int           `json:"totalNodes"`
	ActiveConnections      int64         `json:"activeConnections"`
	UploadBytesPerSecond   float64       `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond float64       `json:"downloadBytesPerSecond"`
	FetchedAt              time.Time     `json:"fetchedAt"`
}
type liveSnapshot struct {
	hello           wire.AgentHello
	report          wire.TelemetryReport
	receivedAt      time.Time
	logs            []json.RawMessage
	firewallPending string
}
