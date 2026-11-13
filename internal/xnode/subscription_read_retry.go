package xnode

import (
	"context"
	"errors"
	"fmt"
)

var errSubscriptionReadContention = fmt.Errorf("subscription read inputs changed: %w", ErrConflict)

// ReadSubscription retries the complete read only after benign input
// contention. Credential/token/period and owner lifecycle authority stay bound
// to the first attempt. The callback must only read or request an idempotent
// refresh; a changed authority always discards the entire returned value.
func ReadSubscription[T any](ctx context.Context, s *Service, identity Identity, read func(Identity) (T, error)) (T, error) {
	var zero T
	expected := subscriptionIdentitySnapshot(identity)
	var owner subscriptionOwnerState
	captured := false
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		if attempt > 0 {
			var err error
			identity, err = s.repo.XNodeIdentity(ctx, expected.proxyID)
			if err != nil {
				return zero, err
			}
		}
		current := subscriptionIdentitySnapshot(identity)
		if current != expected && !subscriptionProofGained(expected, current) {
			return zero, ErrConflict
		}
		fence, err := s.BeginSubscriptionRead(ctx, identity)
		if err != nil && !errors.Is(err, errSubscriptionReadContention) {
			return zero, err
		}
		if captured && owner != fence.owner {
			return zero, ErrConflict
		}
		owner, captured = fence.owner, true
		if err != nil {
			continue
		}
		result, readErr := read(identity)
		if err = s.CheckSubscriptionRead(ctx, fence); errors.Is(err, errSubscriptionReadContention) {
			continue
		}
		if err != nil {
			return zero, err
		}
		return result, readErr
	}
	return zero, ErrConflict
}

func subscriptionProofGained(before, after subscriptionIdentityState) bool {
	if before.proof != "" || after.proof == "" {
		return false
	}
	after.proof = ""
	return before == after
}
