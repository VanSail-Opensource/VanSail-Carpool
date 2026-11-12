package admin

import (
	"context"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

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
	fingerprint, err := commandFingerprint(struct {
		UserID      string
		Replacement ComboReplacement
	}{userID, replacement})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if receipt, found, err := s.repository.ProviderOperationForActorKey(ctx, actorID, providerops.KindAdminComboReplacement,
		strings.TrimSpace(key), fingerprint); found || err != nil {
		return receipt, err
	}
	if err := s.validateComboAndSquads(ctx, replacement.ComboID, replacement.AddonSquadUUIDs); err != nil {
		return model.OperationReceipt{}, err
	}
	return s.repository.ReplaceAdminCombo(ctx, database.AdminComboReplacementInput{
		ActorUserID: actorID, UserID: userID, ComboID: replacement.ComboID,
		IdempotencyKey: strings.TrimSpace(key), RequestFingerprint: fingerprint,
		Reason: replacement.Reason, AddonSquadUUIDs: replacement.AddonSquadUUIDs,
	}, s.now().UTC())
}
