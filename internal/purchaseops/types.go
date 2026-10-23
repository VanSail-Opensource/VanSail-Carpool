// Package purchaseops owns member combo controls, automatic reset settlement and first-term refunds.
package purchaseops

import (
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

var (
	// ErrNotFound hides purchases that are not owned by the requesting member.
	ErrNotFound = errors.New("member purchase not found")
	// ErrIneligible means the purchase no longer satisfies a quoted mutation.
	ErrIneligible = errors.New("member purchase operation is ineligible")
	// ErrIdempotencyConflict means a key was reused for different input.
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with another request")
)

const (
	ReasonNotActive      = "PURCHASE_NOT_ACTIVE"
	ReasonUnsupported    = "RESET_STRATEGY_UNSUPPORTED"
	ReasonInvalidPrice   = "RESET_PRICE_INVALID"
	ReasonNotFirstTerm   = "REFUND_NOT_FIRST_TERM"
	ReasonWindowExpired  = "REFUND_WINDOW_EXPIRED"
	ReasonTrafficUsed    = "REFUND_TRAFFIC_USED"
	ReasonRemoteUnlinked = "XNODE_IDENTITY_REQUIRED"
	ReasonOperationOpen  = "OPERATION_ALREADY_OPEN"
	OperationResetKind   = "purchase_traffic_reset"
	OperationRefundKind  = "purchase_refund"
	ResetOutboxKind      = "purchase_traffic_reset"
	RefundOutboxKind     = "purchase_refund"
	refundWindow         = 24 * time.Hour
)

// PurchaseFacts are immutable and current fields needed by member operations.
type PurchaseFacts struct {
	Purchase       model.Purchase
	CoreGrossMinor int64
	FirstTerm      bool
}

// TrafficResetQuote is the authoritative reset price and eligibility result.
type TrafficResetQuote struct {
	PurchaseID    string      `json:"purchaseId"`
	Eligible      bool        `json:"eligible"`
	ReasonCode    *string     `json:"reasonCode"`
	Price         model.Money `json:"price"`
	ResetStrategy string      `json:"resetStrategy"`
	QuotedAt      time.Time   `json:"quotedAt"`
}

// MemberRefundQuote is the authoritative refund amount and eligibility result.
type MemberRefundQuote struct {
	PurchaseID           string      `json:"purchaseId"`
	Eligible             bool        `json:"eligible"`
	ReasonCode           *string     `json:"reasonCode"`
	Refund               model.Money `json:"refund"`
	QuotedAt             time.Time   `json:"quotedAt"`
	EligibilityExpiresAt *time.Time  `json:"eligibilityExpiresAt"`
}

// RemoteState is the provider state used for mutation reconciliation.
type RemoteState struct {
	UsedTrafficBytes   int64
	LastTrafficResetAt *time.Time
	Quiesced           bool
}

// RefundResult reports whether an independent queued purchase was activated.
type RefundResult struct {
	Successor *model.Purchase
}

func reason(value string) *string { return &value }
