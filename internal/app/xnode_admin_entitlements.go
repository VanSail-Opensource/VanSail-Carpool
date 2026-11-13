package app

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// SyncAdminUserEntitlement addresses native state by its panel owner, including
// members whose historical proxy link is absent or belongs to a retired source.
func (a proxyProvider) SyncAdminUserEntitlement(ctx context.Context, user model.User, desired *model.Purchase) error {
	if user.ID == "" || desired != nil && desired.UserID != user.ID {
		return xnode.ErrInvalid
	}
	identity, err := a.native.IdentityForUser(ctx, user.ID)
	if errors.Is(err, xnode.ErrNotFound) {
		if desired == nil {
			return nil
		}
		identity, err = a.native.EnsureIdentity(ctx, user)
	}
	if err != nil {
		return err
	}
	if identity.UserID != user.ID {
		return xnode.ErrConflict
	}
	id := strconv.FormatInt(identity.ProxyUserID, 10)
	if desired == nil {
		// A paid successor may become effective while the queued administrator
		// action is waiting. Preserve that access and its exact accounting.
		desired, err = a.native.EffectivePurchase(ctx, user.ID)
		if err != nil {
			return err
		}
	}
	if desired == nil {
		// ApplyPurchase checks current desired state again before closure. A
		// concurrently accepted term conflicts instead of being removed.
		return a.native.ApplyPurchase(ctx, id, 0, "", time.Time{})
	}
	return a.native.ApplyPurchase(ctx, id, desired.TrafficLimitBytes, desired.ResetStrategy, desired.ValidUntil)
}

// SetAdminUserDisabled retains an owner-scoped restriction independently of
// historical proxy links and restores the existing paid period on release.
func (a proxyProvider) SetAdminUserDisabled(ctx context.Context, user model.User, operationID string, disabled bool) error {
	if user.ID == "" {
		return xnode.ErrInvalid
	}
	identity, err := a.native.IdentityForUser(ctx, user.ID)
	if errors.Is(err, xnode.ErrNotFound) {
		if !disabled {
			return nil
		}
		identity, err = a.native.EnsureIdentity(ctx, user)
	}
	if err != nil {
		return err
	}
	if identity.UserID != user.ID {
		return xnode.ErrConflict
	}
	if err = a.store.SetAdminTemporaryBanAccess(ctx, user.ID, identity.ProxyUserID, operationID, disabled); err != nil {
		return err
	}
	id := strconv.FormatInt(identity.ProxyUserID, 10)
	if disabled {
		return a.native.CloseForReplacement(ctx, id)
	}
	purchase, err := a.native.EffectivePurchase(ctx, user.ID)
	if err != nil || purchase == nil {
		return err
	}
	return a.native.ResumePurchase(ctx, id, *purchase)
}
