package purchaseops

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"strconv"
	"strings"
)

// SwitchSquad commits an owner-scoped preference with atomic last-squad validation.
// Native reconciliation reads the committed choice; no provider job is created.
func (s *Service) SwitchSquad(ctx context.Context, userID, purchaseID, uuid string, enabled bool, key string) (model.OperationReceipt, error) {
	fingerprint := operationFingerprint(OperationSquadSwitch, purchaseID+":"+uuid+":"+strconv.FormatBool(enabled))
	if receipt, found, err := s.replay(ctx, userID, OperationSquadSwitch, key, fingerprint); found || err != nil {
		return receipt, err
	}
	repository, err := s.controlsRepository()
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, err := repository.BeginSquadSwitch(ctx, command(userID, purchaseID, OperationSquadSwitch, key, fingerprint), purchaseID, uuid, enabled, s.now().UTC())
	return operation.Receipt, err
}

// EarlyActivation previews the earliest queued term, including custom durations.
func (s *Service) EarlyActivation(ctx context.Context, userID, queuedID string) (EarlyActivationQuote, error) {
	repository, err := s.controlsRepository()
	if err != nil {
		return EarlyActivationQuote{}, err
	}
	now := s.now().UTC()
	active, queued, err := repository.ActiveAndQueuedPurchases(ctx, userID, now)
	if err != nil {
		return EarlyActivationQuote{}, err
	}
	if active == nil || active.Status != "active" || queued == nil || queued.ID != queuedID || !queued.ValidUntil.After(queued.ValidFrom) {
		return EarlyActivationQuote{}, ErrIneligible
	}
	conflict, err := repository.ComboControlConflict(ctx, userID)
	return EarlyActivationQuote{CurrentID: active.ID, QueuedID: queued.ID, Eligible: !conflict, CurrentValidUntil: active.ValidUntil, NewValidFrom: now, NewValidUntil: now.Add(queued.ValidUntil.Sub(queued.ValidFrom))}, err
}

// ActivateEarly forfeits the current term without a refund or new rollover credit.
func (s *Service) ActivateEarly(ctx context.Context, userID, currentID, queuedID, confirmation, key string) (model.OperationReceipt, error) {
	confirmation = strings.TrimSpace(confirmation)
	if confirmation != "ACTIVATE NEXT COMBO" && confirmation != "启用下一套餐" {
		return model.OperationReceipt{}, ErrConfirmationRequired
	}
	fingerprint := operationFingerprint(OperationEarlyActivation, currentID+":"+queuedID)
	if receipt, found, err := s.replay(ctx, userID, OperationEarlyActivation, key, fingerprint); found || err != nil {
		return receipt, err
	}
	repository, err := s.controlsRepository()
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, err := repository.BeginEarlyActivation(ctx, command(userID, queuedID, OperationEarlyActivation, key, fingerprint), currentID, queuedID, s.now().UTC())
	return operation.Receipt, err
}
