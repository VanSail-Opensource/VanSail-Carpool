// Package connectivity checks authenticated proxy connectivity and retains diagnostic attempts.
package connectivity

import (
	"context"
	"encoding/json"
	"time"
)

// Config is the atomic, credential-free runtime configuration.
type Config struct {
	ScheduledEnabled     bool   `json:"scheduledEnabled"`
	RemnawaveUserID      int64  `json:"remnawaveUserId"`
	IntervalSeconds      int    `json:"intervalSeconds"`
	TimeoutSeconds       int    `json:"timeoutSeconds"`
	MaxRetries           int    `json:"maxRetries"`
	RetryIntervalSeconds int    `json:"retryIntervalSeconds"`
	ProbeURL             string `json:"probeUrl"`
}

// User contains only safe monitoring-account identity and availability.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Status   string `json:"status"`
}

// Target contains transient upstream metadata and a resolved client configuration.
// Resolved must never be persisted, logged, or returned to a browser.
type Target struct {
	HostUUID       string          `json:"hostUuid"`
	Remark         string          `json:"remark"`
	Address        string          `json:"address"`
	Port           int             `json:"port"`
	Resolved       json.RawMessage `json:"-"`
	NativeOutbound json.RawMessage `json:"-"`
}

// Outcome is a sanitized terminal probe result.
type Outcome struct {
	Status     string   `json:"status"`
	LatencyMS  *float64 `json:"latencyMs"`
	HTTPStatus *int     `json:"httpStatus"`
	ErrorCode  string   `json:"errorCode"`
}

// Attempt records one host check, including retries, or a setup failure without secrets.
type Attempt struct {
	ID              string     `json:"id"`
	RunID           string     `json:"runId"`
	HostUUID        string     `json:"hostUuid"`
	ConfigHash      string     `json:"configHash"`
	RemnawaveUserID int64      `json:"remnawaveUserId"`
	Trigger         string     `json:"trigger"`
	StartedAt       time.Time  `json:"startedAt"`
	FinishedAt      *time.Time `json:"finishedAt"`
	Outcome
}

// Run is the current or most recently completed process-owned batch.
type Run struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	Trigger    string     `json:"trigger"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	Total      int        `json:"total"`
	Completed  int        `json:"completed"`
	ErrorCode  string     `json:"errorCode"`
}

// HostResult associates a latest attempt with safe, transient host metadata.
type HostResult struct {
	HostUUID string   `json:"hostUuid"`
	Remark   string   `json:"remark"`
	Address  string   `json:"address"`
	Port     int      `json:"port"`
	Latest   *Attempt `json:"latest"`
}

// Snapshot reports configuration, progress, and freshness independently of history.
type Snapshot struct {
	Config    Config       `json:"config"`
	User      *User        `json:"user"`
	Run       *Run         `json:"run"`
	Hosts     []HostResult `json:"hosts"`
	Stale     bool         `json:"stale"`
	ErrorCode string       `json:"errorCode"`
}

// HistoryPage is a stable descending page of attempts within the last 24 hours.
type HistoryPage struct {
	Items      []Attempt `json:"items"`
	NextCursor *string   `json:"nextCursor"`
}

// Source resolves the existing test account and its accessible enabled hosts.
type Source interface {
	Resolve(context.Context, string) (User, error)
	Validate(context.Context, int64) error
	Load(context.Context, int64) (User, []Target, error)
}

// Probe executes exactly one authenticated host check.
type Probe interface {
	Check(context.Context, Target, Config) Outcome
}

// Settings reads the existing runtime-setting registry.
type Settings interface {
	Optional(context.Context, string) (string, error)
}

// Repository owns retained attempts, recovery, and stable history pagination.
type Repository interface {
	BeginConnectivityAttempt(context.Context, Attempt) error
	FinishConnectivityAttempt(context.Context, string, Outcome, time.Time) error
	LatestConnectivityAttempts(context.Context, string, time.Time) ([]Attempt, error)
	ConnectivityHistory(context.Context, string, string, int, time.Time) (HistoryPage, error)
	RecoverConnectivityAttempts(context.Context, time.Time) error
	PruneConnectivityAttempts(context.Context, time.Time) error
}
