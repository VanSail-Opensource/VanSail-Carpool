package purchaseops

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"time"
)

const (
	// OperationSquadSwitch changes access without changing purchased ownership.
	OperationSquadSwitch = "member_squad_switch"
	// OperationEarlyActivation forfeits a term and starts its queued successor.
	OperationEarlyActivation = "member_early_activation"
)

var (
	// ErrLastEnabledSquad protects the member's remaining access.
	ErrLastEnabledSquad = errors.New("at least one internal squad must remain enabled")
	// ErrConfirmationRequired rejects an unconfirmed forfeiture.
	ErrConfirmationRequired = errors.New("early activation confirmation is required")
)

// SquadControl is one owned internal squad's current access preference.
type SquadControl struct {
	UUID       string `json:"uuid"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	CanDisable bool   `json:"canDisable"`
}

// ComboControls returns local term ownership and live squad names.
type ComboControls struct {
	Active    *model.Purchase         `json:"activePurchase"`
	Queued    *model.Purchase         `json:"queuedPurchase"`
	Squads    []SquadControl          `json:"squads"`
	Mutable   bool                    `json:"mutable"`
	Operation *model.OperationReceipt `json:"operation"`
}

// EarlyActivationQuote previews a full-duration successor starting now.
type EarlyActivationQuote struct {
	CurrentID         string    `json:"currentPurchaseId"`
	QueuedID          string    `json:"queuedPurchaseId"`
	Eligible          bool      `json:"eligible"`
	CurrentValidUntil time.Time `json:"currentValidUntil"`
	NewValidFrom      time.Time `json:"newValidFrom"`
	NewValidUntil     time.Time `json:"newValidUntil"`
}

type comboControlRepository interface {
	ActiveAndQueuedPurchases(context.Context, string, time.Time) (*model.Purchase, *model.Purchase, error)
	UserDisabledSquads(context.Context, string) ([]string, error)
	LatestComboControl(context.Context, string) (*model.OperationReceipt, error)
	ComboControlConflict(context.Context, string) (bool, error)
	BeginSquadSwitch(context.Context, providerops.CreateInput, string, string, bool, time.Time) (providerops.Operation, error)
	BeginEarlyActivation(context.Context, providerops.CreateInput, string, string, time.Time) (providerops.Operation, error)
}

type squadNameSource interface {
	MemberSquadNames(context.Context) (map[string]string, error)
}

func (s *Service) controlsRepository() (comboControlRepository, error) {
	repository, ok := s.repository.(comboControlRepository)
	if !ok {
		return nil, errors.New("combo controls are unavailable")
	}
	return repository, nil
}
