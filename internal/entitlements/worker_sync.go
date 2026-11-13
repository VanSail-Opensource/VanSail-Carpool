package entitlements

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Native synchronization resolves the stable panel owner even when a legacy
// proxy pointer is missing, stale or belongs to a different account.
type memberEntitlementSynchronizer interface {
	SyncMemberEntitlement(context.Context, model.User, *model.Purchase) error
}

func (w *Worker) syncUser(ctx context.Context, userID string) error {
	user, err := w.repository.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.OnboardingState != "complete" {
		return nil
	}
	if remote, ok := w.provider.(memberEntitlementSynchronizer); ok {
		desired, err := w.repository.DesiredEntitlement(ctx, user.ID, w.now().UTC())
		if err != nil {
			return err
		}
		if err := remote.SyncMemberEntitlement(ctx, user, desired); err != nil {
			return err
		}
		return w.releaseUserSyncNotifications(ctx, userID)
	}
	if repository, ok := w.repository.(provisioningRepository); ok {
		purchased, err := repository.HasProvisionablePurchase(ctx, userID, w.now().UTC())
		if err != nil {
			return err
		}
		if purchased {
			user, err = w.ensureIdentity(ctx, user)
			if errors.Is(err, accounts.ErrProxyIdentityConflict) {
				return w.resolveIdentityConflict(ctx, user)
			}
			if err != nil {
				return err
			}
		}
	}
	if user.ProxyUserID == nil {
		return w.releaseUserSyncNotifications(ctx, userID)
	}
	desired, err := w.repository.DesiredEntitlement(ctx, user.ID, w.now().UTC())
	if err != nil {
		return err
	}
	if desired == nil {
		if err := w.provider.RemoveEntitlement(ctx, *user.ProxyUserID); err != nil {
			return w.repairMissingIdentity(ctx, user, err)
		}
		return w.releaseUserSyncNotifications(ctx, userID)
	}
	if err := w.provider.ApplyEntitlement(ctx, *user.ProxyUserID, desired.TrafficLimitBytes, desired.ResetStrategy, desired.SquadUUIDs, desired.ValidUntil); err != nil {
		return w.repairMissingIdentity(ctx, user, err)
	}
	return w.releaseUserSyncNotifications(ctx, userID)
}
