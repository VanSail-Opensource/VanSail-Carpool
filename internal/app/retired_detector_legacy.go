package app

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type detectorLegacyRepository interface {
	ActiveAdminTemporaryBan(context.Context, string) (*database.AdminTemporaryBan, error)
	DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error)
	EnabledSquadsForRemote(context.Context, string, []string) ([]string, error)
}

func releaseRetiredDetectorLegacy(ctx context.Context, adapter remnaAdapter, repo detectorLegacyRepository, userID, legacyID string, desired func(context.Context) (*model.Purchase, error)) error {
	id, err := remnaUserID(legacyID)
	if err != nil {
		return err
	}
	// Recheck manual ownership and paid access inside the serialized upstream
	// write. A manual ban queued before this operation cannot be overwritten.
	return remnaExecute(ctx, adapter, func(callCtx context.Context, client remnaClient) error {
		manual, e := repo.ActiveAdminTemporaryBan(callCtx, userID)
		if e != nil || manual != nil {
			return e
		}
		var purchase *model.Purchase
		if desired != nil {
			purchase, e = desired(callCtx)
		} else {
			purchase, e = repo.DesiredEntitlement(callCtx, userID, time.Now().UTC())
		}
		if e != nil {
			return e
		}
		status, limit, strategy := remnawave.UserStatusDisabled, int64(0), remnawave.TrafficNoReset
		expires, squads := time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC), []string{}
		if purchase != nil {
			status, limit, strategy = remnawave.UserStatusActive, purchase.TrafficLimitBytes, remnawave.TrafficLimitStrategy(purchase.ResetStrategy)
			expires = purchase.ValidUntil.UTC()
			squads, e = repo.EnabledSquadsForRemote(callCtx, legacyID, purchase.SquadUUIDs)
			if e != nil {
				return e
			}
		}
		_, e = client.UpdateUser(callCtx, remnawave.UpdateUserRequest{ID: id, Status: &status, TrafficLimitBytes: &limit, TrafficLimitStrategy: &strategy, ExpireAt: &expires, ActiveInternalSquads: &squads, ClearExternalSquad: true})
		return e
	})
}
