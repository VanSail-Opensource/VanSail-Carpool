package admin

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// AdminEntitlementCreate describes one zero-charge, administrator-owned manual
// fixed-term grant. It never debits the wallet or creates ledger debt.
type AdminEntitlementCreate struct {
	Reason            string
	ComboID           string
	ValidFrom         time.Time
	ValidUntil        time.Time
	TrafficLimitBytes int64
	ResetStrategy     string
	SquadUUIDs        []string
}

// adminEntitlementCreator is the narrow store command used by manual grants.
type adminEntitlementCreator interface {
	CreateAdminEntitlement(context.Context, database.AdminEntitlementCreateInput, time.Time) (model.Purchase, error)
}

// CreateEntitlement validates a manual grant and inserts it as one audited,
// queued, zero-charge purchase. Omitted squads inherit the combo's included
// squads; every squad must exist upstream.
func (s *UserWorkflows) CreateEntitlement(ctx context.Context, actorID, userID, key string, grant AdminEntitlementCreate) (model.Purchase, error) {
	grant.Reason = strings.TrimSpace(grant.Reason)
	if !validCommand(actorID, key, grant.Reason) || userID == "" || strings.TrimSpace(grant.ComboID) == "" ||
		grant.ValidUntil.Sub(grant.ValidFrom) < time.Second || grant.TrafficLimitBytes <= 0 || !validResetStrategy(grant.ResetStrategy) {
		return model.Purchase{}, errors.New("invalid entitlement grant")
	}
	squads, err := normalizeUUIDs(grant.SquadUUIDs)
	if err != nil {
		return model.Purchase{}, err
	}
	grant.ComboID = strings.TrimSpace(grant.ComboID)
	combo, err := s.repository.ComboByID(ctx, grant.ComboID, true)
	if err != nil {
		return model.Purchase{}, err
	}
	if len(squads) == 0 {
		for _, included := range combo.IncludedSquads {
			squads = append(squads, included.GroupID)
		}
		if squads, err = normalizeUUIDs(squads); err != nil {
			return model.Purchase{}, err
		}
	}
	if err := s.validateImportedSquads(ctx, squads); err != nil {
		return model.Purchase{}, err
	}
	grant.SquadUUIDs = squads
	grant.ValidFrom, grant.ValidUntil = grant.ValidFrom.UTC(), grant.ValidUntil.UTC()
	fingerprint, err := commandFingerprint(struct {
		ActorID, UserID, Key string
		Grant                AdminEntitlementCreate
	}{actorID, userID, strings.TrimSpace(key), grant})
	if err != nil {
		return model.Purchase{}, err
	}
	creator, ok := s.repository.(adminEntitlementCreator)
	if !ok {
		return model.Purchase{}, errors.New("entitlement creation is unavailable")
	}
	return creator.CreateAdminEntitlement(ctx, database.AdminEntitlementCreateInput{
		ActorUserID: actorID, UserID: userID, IdempotencyKey: strings.TrimSpace(key),
		RequestFingerprint: fingerprint, Reason: grant.Reason, ComboID: grant.ComboID,
		ValidFrom: grant.ValidFrom, ValidUntil: grant.ValidUntil, TrafficLimitBytes: grant.TrafficLimitBytes,
		ResetStrategy: grant.ResetStrategy, SquadUUIDs: grant.SquadUUIDs,
	}, s.now().UTC())
}

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
