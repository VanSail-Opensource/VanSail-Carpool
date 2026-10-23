// Package compensation detects Proxy node outages and gates extensions behind review.
package compensation

import (
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const (
	MaxMinutes       = 5_256_000
	MinMultiplierBPS = 100
	MaxMultiplierBPS = 1_000_000
)

var (
	ErrInvalid  = errors.New("invalid node compensation input")
	ErrConflict = errors.New("node compensation state conflict")
)

type Config struct {
	Enabled          bool      `json:"enabled"`
	ThresholdMinutes *int      `json:"thresholdMinutes"`
	MultiplierBPS    *int      `json:"multiplierBps"`
	Revision         int       `json:"revision"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type ConfigUpdate struct {
	Enabled          bool
	ThresholdMinutes *int
	MultiplierBPS    *int
	Revision         int
}

type Squad struct {
	UUID     string   `json:"uuid"`
	Name     string   `json:"name"`
	Inbounds []string `json:"-"`
}

type Node struct {
	UUID, Name          string
	Connected, Disabled bool
	ActiveInboundUUIDs  []string
	AffectedSquads      []Squad
}

type Event struct {
	ID                       string                  `json:"id"`
	NodeUUID                 string                  `json:"nodeUuid"`
	NodeName                 string                  `json:"nodeName"`
	Status                   string                  `json:"status"`
	OfflineObservedAt        time.Time               `json:"offlineObservedAt"`
	RecoveredObservedAt      *time.Time              `json:"recoveredObservedAt"`
	ObservedDurationSeconds  *int64                  `json:"observedDurationSeconds"`
	ThresholdMinutes         int                     `json:"thresholdMinutes"`
	MultiplierBPS            int                     `json:"multiplierBps"`
	ProposedExtensionMinutes *int                    `json:"proposedExtensionMinutes"`
	FinalExtensionMinutes    *int                    `json:"finalExtensionMinutes"`
	Capped                   bool                    `json:"capped"`
	Squads                   []Squad                 `json:"squads"`
	FrozenRecipientCount     int                     `json:"frozenRecipientCount"`
	EligibleRecipientCount   *int                    `json:"eligibleRecipientCount"`
	SkippedRecipientCount    *int                    `json:"skippedRecipientCount"`
	ReviewedBy               *string                 `json:"reviewedBy"`
	ReviewedAt               *time.Time              `json:"reviewedAt"`
	ReviewReason             *string                 `json:"reviewReason"`
	IneligibleReason         *string                 `json:"ineligibleReason"`
	Revision                 int                     `json:"revision"`
	Operation                *model.OperationReceipt `json:"operation"`
}

type EventPage struct {
	Items      []Event `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

type ReviewInput struct {
	EventID, ActorUserID, IdempotencyKey, Reason string
	Revision, ExtensionMinutes                   int
}
