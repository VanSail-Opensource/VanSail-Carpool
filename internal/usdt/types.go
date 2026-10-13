// Package usdt owns watch-only USDT collection, quoted amounts and queued chain readers.
package usdt

import (
	"context"
	"encoding/json"
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
	Enabled       bool                     `json:"enabled"`
	Name          string                   `json:"name"`
	Rate          string                   `json:"rate"`
	ExpiryMinutes int                      `json:"expiryMinutes"`
	Concurrency   int                      `json:"concurrency"`
	Networks      []NetworkConfig          `json:"networks"`
	LegacyGroups  []legacyConcurrencyGroup `json:"groups,omitempty"`
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
	LegacyGroup       string `json:"group,omitempty"`
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

type legacyConcurrencyGroup map[string]json.RawMessage

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

type Status struct {
	Networks []NetworkStatus `json:"networks"`
}
type ReviewPage struct {
	Items      []Transfer `json:"items"`
	NextCursor string     `json:"nextCursor"`
}

type Repository interface {
	ReserveUSDTQuote(context.Context, string, NetworkSpec, []string, string) (Quote, error)
	USDTQuote(context.Context, string) (Quote, error)
	USDTWatchAddresses(context.Context, string) ([]string, error)
	USDTCursor(context.Context, string) (Cursor, error)
	SaveUSDTBatch(context.Context, string, Batch) error
	SettleUSDTTransfer(context.Context, string, string, string, string) (model.PaymentOrder, bool, error)
	USDTReview(context.Context, string, int) (ReviewPage, error)
	USDTReady(context.Context, int) ([]Transfer, error)
}

type Settings interface {
	Optional(context.Context, string) (string, error)
	Put(context.Context, string, string, string) error
}
