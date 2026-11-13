package app

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Preserve the authority authenticated before native reads begin. The native
// read fence protects changes during projection; it cannot recover an earlier
// request generation after an account has already been purged and revived.
func (a proxyProvider) readAuthenticatedSubscription(ctx context.Context, user model.User, read func() (subscription.MemberSubscription, error)) (subscription.MemberSubscription, error) {
	check := func() error {
		if a.store == nil || user.ID == "" || user.AccountGeneration < 0 {
			return xnode.ErrInvalid
		}
		current, err := a.store.UserByID(ctx, user.ID)
		if err != nil {
			return err
		}
		if current.ID != user.ID || current.AccountGeneration != user.AccountGeneration || current.AccountStatus != "active" || current.BotBlocked {
			return xnode.ErrConflict
		}
		return nil
	}
	if err := check(); err != nil {
		return subscription.MemberSubscription{}, err
	}
	result, err := read()
	if err != nil {
		return subscription.MemberSubscription{}, err
	}
	if err := check(); err != nil {
		return subscription.MemberSubscription{}, err
	}
	return result, nil
}
