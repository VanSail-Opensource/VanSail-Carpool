// Package usdt owns watch-only USDT collection, quoted amounts and queued chain readers.
package usdt

import (
	"context"
	"errors"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

const Provider = "builtin_usdt"
const ConfigKey = "billing.usdt.config"

var (
	ErrConfiguration = errors.New("USDT_CONFIGURATION_INVALID")
	ErrUnavailable   = errors.New("USDT_NETWORK_UNAVAILABLE")
	ErrCapacity      = errors.New("USDT_QUOTE_CAPACITY")
	ErrConflict      = errors.New("USDT_TRANSFER_CONFLICT")
	ErrRPC           = errors.New("USDT_RPC_UNAVAILABLE")
	ErrProof         = errors.New("USDT_TRANSFER_UNVERIFIED")
	ErrQuoteNotFound = errors.New("USDT_QUOTE_NOT_FOUND")
)

// Config is encrypted as one setting; PublicConfig removes endpoint secrets.
type Config struct {
	Enabled       bool            `json:"enabled"`
	Name          string          `json:"name"`
	Rate          string          `json:"rate"`
	ExpiryMinutes int             `json:"expiryMinutes"`
	Networks      []NetworkConfig `json:"networks"`
	Groups        []QuotaGroup    `json:"groups"`
}

type NetworkConfig struct {
	Network   string     `json:"network"`
	Enabled   bool       `json:"enabled"`
	Addresses []string   `json:"addresses"`
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint credentials and URLs are write-only in the administrative projection.
type Endpoint struct {
	ID                string `json:"id"`
	Group             string `json:"group"`
	RPCURL            string `json:"rpcUrl"`
	WSURL             string `json:"wsUrl"`
	IndexerURL        string `json:"indexerUrl"`
	APIKey            string `json:"apiKey"`
	UseWS             bool   `json:"useWs"`
	RPCConfigured     bool   `json:"rpcConfigured"`
	WSConfigured      bool   `json:"wsConfigured"`
	IndexerConfigured bool   `json:"indexerConfigured"`
	KeyConfigured     bool   `json:"keyConfigured"`
	Host              string `json:"host"`
}

type QuotaGroup struct {
	ID                string `json:"id"`
	RequestsPerSecond int    `json:"requestsPerSecond"`
	RequestsPerMinute int    `json:"requestsPerMinute"`
	RequestsPerDay    int    `json:"requestsPerDay"`
	Concurrency       int    `json:"concurrency"`
}

type Quote struct {
	OrderID      string    `json:"orderId"`
	Network      string    `json:"network"`
	Token        string    `json:"token"`
	Recipient    string    `json:"recipient"`
	Amount       string    `json:"amount"`
	AtomicAmount string    `json:"atomicAmount"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// Transfer records one receiving event, never a transaction-wide balance delta.
type Transfer struct {
	ID           string    `json:"id"`
	Network      string    `json:"network"`
	Token        string    `json:"token"`
	Recipient    string    `json:"recipient"`
	AtomicAmount string    `json:"atomicAmount"`
	Amount       string    `json:"amount"`
	TxHash       string    `json:"txHash"`
	EventID      string    `json:"eventId"`
	OccurredAt   time.Time `json:"occurredAt"`
	Finalized    bool      `json:"-"`
	Verified     bool      `json:"-"`
	OrderID      string    `json:"orderId"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason"`
}

type Cursor struct {
	Value string
	Hash  string
}
type Batch struct {
	Transfers []Transfer
	Cursor    Cursor
	CaughtUp  bool
}

type NetworkStatus struct {
	Network    string     `json:"network"`
	State      string     `json:"state"`
	Cursor     string     `json:"cursor"`
	UpdatedAt  *time.Time `json:"updatedAt"`
	ErrorCode  string     `json:"errorCode"`
	Connection string     `json:"connection"`
}

type QuotaStatus struct {
	ID            string     `json:"id"`
	MinuteUsed    int        `json:"minuteUsed"`
	DayUsed       int        `json:"dayUsed"`
	CooldownUntil *time.Time `json:"cooldownUntil"`
}

type Status struct {
	Networks []NetworkStatus `json:"networks"`
	Quotas   []QuotaStatus   `json:"quotas"`
}
type ReviewPage struct {
	Items      []Transfer `json:"items"`
	NextCursor string     `json:"nextCursor"`
}

// Pause indicates admission was denied without sending an upstream request.
type Pause struct {
	Until time.Time
	Code  string
}

func (e *Pause) Error() string { return e.Code }

type Repository interface {
	ReserveUSDTQuote(context.Context, string, NetworkSpec, []string, string) (Quote, error)
	USDTQuote(context.Context, string) (Quote, error)
	USDTWatchAddresses(context.Context, string) ([]string, error)
	USDTCursor(context.Context, string) (Cursor, error)
	SaveUSDTBatch(context.Context, string, Batch) error
	SettleUSDTTransfer(context.Context, string, string, string, string) (model.PaymentOrder, bool, error)
	USDTReview(context.Context, string, int) (ReviewPage, error)
	USDTReady(context.Context, int) ([]Transfer, error)
	AdmitUSDTRPC(context.Context, QuotaGroup, time.Time) error
	FinishUSDTRPC(context.Context, string, time.Time, bool) error
	USDTQuotaStatus(context.Context, []QuotaGroup, time.Time) ([]QuotaStatus, error)
}

type Settings interface {
	Optional(context.Context, string) (string, error)
	Put(context.Context, string, string, string) error
}
