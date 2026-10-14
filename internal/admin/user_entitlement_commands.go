package admin

import (
	"context"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// EditEntitlement validates and atomically queues one optimistic full edit.
func (s *UserWorkflows) EditEntitlement(ctx context.Context, actorID, userID, purchaseID, key string, edit EntitlementEdit) (model.Purchase, error) {
	edit.Reason = strings.TrimSpace(edit.Reason)
	if !validCommand(actorID, key, edit.Reason) || userID == "" || purchaseID == "" || edit.Version.IsZero() ||
		!edit.ValidUntil.After(edit.ValidFrom) || edit.TrafficLimitBytes <= 0 || !validEntitlementStatus(edit.Status) || !validResetStrategy(edit.ResetStrategy) {
		return model.Purchase{}, errors.New("invalid entitlement edit")
	}
	var err error
	edit.SquadUUIDs, err = normalizeUUIDs(edit.SquadUUIDs)
	if err != nil {
		return model.Purchase{}, err
	}
	edit.ComboID = strings.TrimSpace(edit.ComboID)
	if err := s.validateComboAndSquads(ctx, edit.ComboID, edit.SquadUUIDs); err != nil {
		return model.Purchase{}, err
	}
	edit.ValidFrom, edit.ValidUntil, edit.Version = edit.ValidFrom.UTC(), edit.ValidUntil.UTC(), edit.Version.UTC()
	fingerprint, err := commandFingerprint(struct {
		UserID, PurchaseID, Key string
		Edit                    EntitlementEdit
	}{userID, purchaseID, strings.TrimSpace(key), edit})
	if err != nil {
		return model.Purchase{}, err
	}
	return s.repository.EditAdminEntitlement(ctx, database.AdminEntitlementEditInput{
		ActorUserID: actorID, UserID: userID, PurchaseID: purchaseID, IdempotencyKey: strings.TrimSpace(key),
		RequestFingerprint: fingerprint, Reason: edit.Reason, ExpectedUpdatedAt: edit.Version,
		ComboID: edit.ComboID, ValidFrom: edit.ValidFrom, ValidUntil: edit.ValidUntil, Status: edit.Status,
		TrafficLimitBytes: edit.TrafficLimitBytes, ResetStrategy: edit.ResetStrategy, SquadUUIDs: edit.SquadUUIDs,
	}, s.now().UTC())
}

// RefundEntitlement credits a validated portion of the original net debit and queues exact provider sync.
func (s *UserWorkflows) RefundEntitlement(ctx context.Context, actorID, userID, purchaseID, key, reason string, amountTXBMinor int64) (model.OperationReceipt, error) {
	reason = strings.TrimSpace(reason)
	if !validCommand(actorID, key, reason) || userID == "" || purchaseID == "" || amountTXBMinor <= 0 {
		return model.OperationReceipt{}, errors.New("invalid entitlement refund")
	}
	fingerprint, err := commandFingerprint(struct {
		UserID, PurchaseID, Reason string
		AmountTXBMinor             int64
	}{userID, purchaseID, reason, amountTXBMinor})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	return s.repository.RefundAdminEntitlement(ctx, database.AdminEntitlementRefundInput{
		ActorUserID: actorID, UserID: userID, PurchaseID: purchaseID, IdempotencyKey: strings.TrimSpace(key),
		RequestFingerprint: fingerprint, Reason: reason, AmountTXBMinor: amountTXBMinor,
	}, s.now().UTC())
}

// ReplaceCombo queues a no-charge active configuration replacement.
func (s *UserWorkflows) ReplaceCombo(ctx context.Context, actorID, userID, key string, replacement ComboReplacement) (model.OperationReceipt, error) {
	replacement.Reason = strings.TrimSpace(replacement.Reason)
	if !validCommand(actorID, key, replacement.Reason) || userID == "" {
		return model.OperationReceipt{}, errors.New("invalid combo replacement")
	}
	var err error
	replacement.AddonSquadUUIDs, err = normalizeUUIDs(replacement.AddonSquadUUIDs)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	replacement.ComboID = strings.TrimSpace(replacement.ComboID)
	if err := s.validateComboAndSquads(ctx, replacement.ComboID, replacement.AddonSquadUUIDs); err != nil {
		return model.OperationReceipt{}, err
	}
	fingerprint, err := commandFingerprint(struct {
		UserID      string
		Replacement ComboReplacement
	}{userID, replacement})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	return s.repository.ReplaceAdminCombo(ctx, database.AdminComboReplacementInput{
		ActorUserID: actorID, UserID: userID, ComboID: replacement.ComboID,
		IdempotencyKey: strings.TrimSpace(key), RequestFingerprint: fingerprint,
		Reason: replacement.Reason, AddonSquadUUIDs: replacement.AddonSquadUUIDs,
	}, s.now().UTC())
}

func validEntitlementStatus(value string) bool {
	switch value {
	case "activating", "active", "queued", "expired", "cancelled", "failed":
		return true
	default:
		return false
	}
}

func validResetStrategy(value string) bool {
	return value == "DAY" || value == "WEEK" || value == "MONTH_ROLLING"
}
