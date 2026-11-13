package admin

import (
	"context"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
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
	edit.ValidFrom, edit.ValidUntil, edit.Version = edit.ValidFrom.UTC(), edit.ValidUntil.UTC(), edit.Version.UTC()
	fingerprint, err := commandFingerprint(struct {
		UserID, PurchaseID, Key string
		Edit                    EntitlementEdit
	}{userID, purchaseID, strings.TrimSpace(key), edit})
	if err != nil {
		return model.Purchase{}, err
	}
	if _, found, err := s.repository.ProviderOperationForActorKey(ctx, actorID, providerops.KindAdminEntitlementEdit,
		strings.TrimSpace(key), fingerprint); err != nil {
		return model.Purchase{}, err
	} else if !found {
		if err := s.validateComboAndSquads(ctx, edit.ComboID, edit.SquadUUIDs); err != nil {
			return model.Purchase{}, err
		}
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

// validateImportedSquads checks freshly imported squads against the upstream
// list. The durable command repeats group existence inside its transaction.
func (s *UserWorkflows) validateImportedSquads(ctx context.Context, squads []string) error {
	if len(squads) == 0 {
		return nil
	}
	if s.importer == nil {
		return errors.New("squad validation is unavailable")
	}
	live, err := s.importer.ListInternalSquads(ctx)
	if err != nil {
		return err
	}
	available := make(map[string]struct{}, len(live))
	for _, squad := range live {
		available[strings.ToLower(squad.UUID)] = struct{}{}
	}
	for _, squad := range squads {
		if _, exists := available[strings.ToLower(squad)]; !exists {
			return database.ErrNotFound
		}
	}
	return nil
}
