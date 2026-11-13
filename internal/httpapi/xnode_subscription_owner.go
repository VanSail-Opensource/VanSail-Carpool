package httpapi

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Native reads capture their owner generation when projection starts. Preserve
// the earlier authenticated authority too, so an in-flight request cannot read
// credentials minted after the account was purged and revived.
func (s *Server) readAuthenticatedNativeSubscription(ctx context.Context, user model.User, read func() (xnode.SubscriptionMetadata, error)) (xnode.SubscriptionMetadata, error) {
	check := func() error {
		if s.deps.Store == nil || user.ID == "" || user.AccountGeneration < 0 {
			return xnode.ErrInvalid
		}
		current, err := s.deps.Store.UserByID(ctx, user.ID)
		if err != nil {
			return err
		}
		if current.ID != user.ID || current.AccountGeneration != user.AccountGeneration || current.AccountStatus != "active" || current.BotBlocked {
			return xnode.ErrConflict
		}
		return nil
	}
	if err := check(); err != nil {
		return xnode.SubscriptionMetadata{}, err
	}
	value, err := read()
	if err != nil {
		return xnode.SubscriptionMetadata{}, err
	}
	if err := check(); err != nil {
		return xnode.SubscriptionMetadata{}, err
	}
	return value, nil
}
