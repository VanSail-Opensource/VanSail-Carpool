package admin

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type commandReplayRecord struct {
	fingerprint string
	receipt     model.OperationReceipt
}

type commandReplayRepository struct {
	userWorkflowRepositoryStub
	accepted   map[string]commandReplayRecord
	lookupErr  error
	comboErr   error
	comboReads int
	applied    int
	editCalls  int
}

func (r *commandReplayRepository) ProviderOperationForActorKey(_ context.Context, actor, kind, key, fingerprint string) (model.OperationReceipt, bool, error) {
	if r.lookupErr != nil {
		return model.OperationReceipt{}, false, r.lookupErr
	}
	record, found := r.accepted[actor+"\x00"+kind+"\x00"+key]
	if !found {
		return model.OperationReceipt{}, false, nil
	}
	if record.fingerprint != fingerprint {
		return record.receipt, true, database.ErrConflict
	}
	return record.receipt, true, nil
}

func (r *commandReplayRepository) ComboByID(context.Context, string, bool) (model.Combo, error) {
	r.comboReads++
	return model.Combo{ID: "combo-1"}, r.comboErr
}

func (r *commandReplayRepository) accept(actor, kind, key, fingerprint string) (model.OperationReceipt, error) {
	receipt, found, err := r.ProviderOperationForActorKey(context.Background(), actor, kind, key, fingerprint)
	if found || err != nil {
		return receipt, err
	}
	if r.accepted == nil {
		r.accepted = make(map[string]commandReplayRecord)
	}
	receipt = model.OperationReceipt{ID: "accepted-operation", Kind: kind, Status: "succeeded"}
	r.accepted[actor+"\x00"+kind+"\x00"+key] = commandReplayRecord{fingerprint: fingerprint, receipt: receipt}
	r.applied++
	return receipt, nil
}

func (r *commandReplayRepository) EditAdminEntitlement(_ context.Context, input database.AdminEntitlementEditInput, _ time.Time) (model.Purchase, error) {
	r.editCalls++
	_, err := r.accept(input.ActorUserID, providerops.KindAdminEntitlementEdit, input.IdempotencyKey, input.RequestFingerprint)
	return model.Purchase{ID: input.PurchaseID, UserID: input.UserID}, err
}

func (r *commandReplayRepository) ReplaceAdminCombo(_ context.Context, input database.AdminComboReplacementInput, _ time.Time) (model.OperationReceipt, error) {
	return r.accept(input.ActorUserID, providerops.KindAdminComboReplacement, input.IdempotencyKey, input.RequestFingerprint)
}

func (r *commandReplayRepository) CreateAdminBulkExtension(_ context.Context, input database.AdminBulkExtensionInput, _ time.Time) (model.OperationReceipt, error) {
	return r.accept(input.ActorUserID, providerops.KindAdminBulkExtension, input.IdempotencyKey, input.RequestFingerprint)
}

func replayCommand(ctx context.Context, workflows *UserWorkflows, kind, actor, owner, key string, changed bool) (string, error) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	switch kind {
	case providerops.KindAdminEntitlementEdit:
		traffic := int64(1_000)
		if changed {
			traffic++
		}
		purchase, err := workflows.EditEntitlement(ctx, actor, owner, "purchase-1", key, EntitlementEdit{
			Version: now, Reason: "operator request", ComboID: " combo-1 ", ValidFrom: now,
			ValidUntil: now.Add(time.Hour), Status: "active", TrafficLimitBytes: traffic,
			ResetStrategy: "DAY", SquadUUIDs: []string{manualGrantSquadID, manualGrantSquadID},
		})
		return purchase.ID, err
	case providerops.KindAdminComboReplacement:
		reason := "operator request"
		if changed {
			reason += " changed"
		}
		receipt, err := workflows.ReplaceCombo(ctx, actor, owner, key, ComboReplacement{
			ComboID: " combo-1 ", AddonSquadUUIDs: []string{manualGrantSquadID, manualGrantSquadID}, Reason: reason,
		})
		return receipt.ID, err
	default:
		minutes := 30
		if changed {
			minutes++
		}
		receipt, err := workflows.CreateBulkExtension(ctx, actor, key, BulkExtension{
			ComboIDs: []string{"combo-1", " combo-1 "}, AddonSquadUUIDs: []string{manualGrantSquadID},
			DurationMinutes: minutes, Reason: "operator request",
		})
		return receipt.ID, err
	}
}
