package entitlements

import (
	"context"
	"errors"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func (w *Worker) prepareContinuity(ctx context.Context, job model.OutboxJob) error {
	successorID, err := jobpayload.TargetID(job, "purchaseId")
	if err != nil {
		return err
	}
	current, err := w.repository.ContinuityEntitlement(ctx, successorID, w.now().UTC())
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	user, err := w.repository.UserForPurchase(ctx, current.ID)
	if err != nil {
		return err
	}
	user, err = w.ensureIdentity(ctx, user)
	if errors.Is(err, accounts.ErrRemnawaveIdentityConflict) {
		return w.resolveIdentityConflict(ctx, user)
	}
	if err != nil {
		return err
	}
	if user.RemnaUserID == nil {
		return errors.New("user has no Remnawave identity")
	}
	if err := w.remnawave.ApplyEntitlement(ctx, *user.RemnaUserID, current.TrafficLimitBytes, current.ResetStrategy, current.SquadUUIDs, current.ValidUntil); err != nil {
		return w.repairMissingIdentity(ctx, user, fmt.Errorf("prepare Remnawave entitlement continuity: %w", err))
	}
	return nil
}
