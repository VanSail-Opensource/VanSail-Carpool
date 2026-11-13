package xnode

import (
	"context"
	"crypto/sha256"
	"errors"
	"slices"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ApplyMemberPurchase applies only the exact paid target of an activation job.
// A successor with identical price/limits/expiry still has a different identity.
func (s *Service) ApplyMemberPurchase(ctx context.Context, user model.User, purchase model.Purchase) error {
	if user.ID == "" || user.AccountGeneration < 0 || purchase.ID == "" || purchase.UserID != user.ID || purchase.UpdatedAt.IsZero() {
		return ErrInvalid
	}
	now := s.now().UTC()
	if purchase.ValidFrom.After(now) || !purchase.ValidUntil.After(now) ||
		(purchase.Status != "activating" && purchase.Status != "active" && purchase.Status != "failed") {
		return ErrConflict
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.repo.UserByID(ctx, user.ID)
	if err != nil {
		return err
	}
	if current.ID != user.ID || current.AccountStatus != "active" || current.AccountGeneration != user.AccountGeneration || current.OnboardingState != "complete" || current.TelegramID <= 0 {
		return ErrConflict
	}
	identity, err := s.IdentityForUser(ctx, user.ID)
	if errors.Is(err, ErrNotFound) || err == nil && (identity.CredentialsCipher == "" || identity.SubscriptionCipher == "" || len(identity.SubscriptionHash) != sha256.Size) {
		if current.BotBlocked {
			return ErrConflict
		}
		identity, err = s.ensureMemberIdentity(ctx, current)
	}
	if err != nil {
		return err
	}
	if identity.UserID != user.ID || identity.ServiceName != "" {
		return ErrConflict
	}
	_, err = s.applyPurchaseLocked(ctx, strconv.FormatInt(identity.ProxyUserID, 10), purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.ValidUntil, &purchase, user.AccountGeneration)
	return err
}

func sameMemberPurchaseTarget(current, expected model.Purchase) bool {
	left, right := slices.Clone(current.SquadUUIDs), slices.Clone(expected.SquadUUIDs)
	slices.Sort(left)
	slices.Sort(right)
	return current.ID == expected.ID && current.UserID == expected.UserID && current.ComboID == expected.ComboID &&
		current.Status == expected.Status && current.UpdatedAt.Equal(expected.UpdatedAt) && current.ValidFrom.Equal(expected.ValidFrom) &&
		current.ValidUntil.Equal(expected.ValidUntil) && current.TrafficLimitBytes == expected.TrafficLimitBytes &&
		current.ResetStrategy == expected.ResetStrategy && slices.Equal(left, right)
}
