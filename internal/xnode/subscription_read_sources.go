package xnode

import (
	"context"
	"crypto/sha256"
	"time"
)

type subscriptionIdentityState struct {
	proxyID                               int64
	user, service, paygOwner, hourlyOwner string
	username, period, proof               string
	disabled                              bool
	credentials, tokenCipher, tokenHash   [sha256.Size]byte
}

func subscriptionIdentitySnapshot(identity Identity) subscriptionIdentityState {
	state := subscriptionIdentityState{
		proxyID:     identity.ProxyUserID,
		user:        identity.UserID,
		service:     identity.ServiceName,
		paygOwner:   identity.PAYGOwnerID,
		hourlyOwner: identity.HourlyOwnerID,
		username:    identity.Username,
		period:      identity.CurrentPeriodID,
		disabled:    identity.AccessDisabled,
		credentials: sha256.Sum256([]byte(identity.CredentialsCipher)),
		tokenCipher: sha256.Sum256([]byte(identity.SubscriptionCipher)),
		tokenHash:   sha256.Sum256(identity.SubscriptionHash),
	}
	if identity.FirstSyncedAt != nil {
		state.proof = identity.FirstSyncedAt.UTC().Format(time.RFC3339Nano)
	}
	return state
}

type subscriptionOwnerState struct {
	id, status, onboarding string
	generation             int64
	blocked, restricted    bool
}

func (s *Service) subscriptionReadOwner(ctx context.Context, identity Identity, now time.Time) (subscriptionOwnerState, time.Time, error) {
	if identity.UserID != "" {
		owner, err := s.repo.UserByID(ctx, identity.UserID)
		if err != nil {
			return subscriptionOwnerState{}, time.Time{}, err
		}
		restricted, err := s.repo.XNodeUserRestricted(ctx, identity.UserID)
		if err != nil {
			return subscriptionOwnerState{}, time.Time{}, err
		}
		return subscriptionOwnerState{
			id:         owner.ID, status: owner.AccountStatus, onboarding: owner.OnboardingState,
			generation: owner.AccountGeneration, blocked: owner.BotBlocked, restricted: restricted,
		}, time.Time{}, nil
	}
	access, err := s.ServiceAccess(ctx, identity)
	if err != nil {
		return subscriptionOwnerState{}, time.Time{}, err
	}
	if access == nil {
		return subscriptionOwnerState{}, time.Time{}, ErrNotFound
	}
	var boundary time.Time
	for _, at := range []time.Time{access.StartedAt, access.ExpiresAt} {
		if at.After(now) && (boundary.IsZero() || at.Before(boundary)) {
			boundary = at
		}
	}
	return subscriptionOwnerState{}, boundary, nil
}
