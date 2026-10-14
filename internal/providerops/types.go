// Package providerops defines durable, provider-agnostic operation metadata.
package providerops

import (
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Status is a durable operation lifecycle state.
type Status string

const (
	StatusQueued        Status = "queued"
	StatusProcessing    Status = "processing"
	StatusSucceeded     Status = "succeeded"
	StatusFailed        Status = "failed"
	StatusCompensated   Status = "compensated"
	StatusPendingReview Status = "pending_review"
	StatusPartial       Status = "partial"
)

// ItemInput identifies one bounded target in a bulk operation.
type ItemInput struct {
	Key        string
	TargetType string
	TargetID   string
}

// CreateInput is the idempotent command envelope persisted before queueing.
type CreateInput struct {
	ActorUserID        string
	OwnerUserID        string
	Kind               string
	IdempotencyKey     string
	RequestFingerprint string
	SealedTarget       string
	Items              []ItemInput
}

// Operation contains internal replay and provider-attempt metadata.
type Operation struct {
	Receipt            model.OperationReceipt
	ActorUserID        string
	OwnerUserID        string
	IdempotencyKey     string
	RequestFingerprint string
	Attempts           int
	AttemptStartedAt   *time.Time
	ProviderReference  string
	ResultJSON         string
}

// Completion is a terminal or review-required operation result.
type Completion struct {
	Status            Status
	ProviderReference string
	ErrorCode         string
	ResultJSON        string
}

// Item is the durable state of one bounded operation target.
type Item struct {
	OperationID       string
	Key               string
	TargetType        string
	TargetID          string
	Status            Status
	ProviderReference string
	ErrorCode         string
	ResultJSON        string
	AttemptStartedAt  *time.Time
	CompletedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
