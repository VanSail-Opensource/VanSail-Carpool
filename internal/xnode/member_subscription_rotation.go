package xnode

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// RevokeSubscriptionForMember resolves the authenticated owner and binds the
// rotation to its admitted lifecycle generation inside the storage transaction.
// Mutable legacy proxy links never select the credential to rotate.
func (s *Service) RevokeSubscriptionForMember(ctx context.Context, user model.User) (string, error) {
	if user.ID == "" || user.AccountGeneration < 0 {
		return "", ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.IdentityForUser(ctx, user.ID)
	if err != nil {
		return "", err
	}
	if identity.UserID != user.ID || identity.ServiceName != "" {
		return "", ErrConflict
	}
	repo, ok := s.repo.(interface {
		RotateXNodeMemberSubscription(context.Context, Identity, int64, []byte, string) (Identity, error)
	})
	if !ok {
		return "", ErrUnavailable
	}
	next := identity
	if err = s.newSubscription(&next); err != nil {
		return "", err
	}
	identity, err = repo.RotateXNodeMemberSubscription(ctx, identity, user.AccountGeneration, next.SubscriptionHash, next.SubscriptionCipher)
	if err != nil {
		return "", err
	}
	return ReadSubscription(ctx, s, identity, func(current Identity) (string, error) {
		return s.SubscriptionURL(current)
	})
}
