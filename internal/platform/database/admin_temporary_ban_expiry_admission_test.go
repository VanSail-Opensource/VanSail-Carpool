package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestTemporaryBanExpiryContinuesStoredAuthorityAfterActorChanges(t *testing.T) {
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"demoted", "user", "active", 0}, {"purging", "admin", "purging", 0}, {"blocked", "admin", "active", 1},
	} {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			actor, owner := createTestAdmin(t, store, 969015), createTestUser(t, store, 969016)
			now := time.Date(2026, 11, 13, 22, 0, 0, 250_000_000, time.UTC)
			ban, err := store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: actor.ID, UserID: owner.ID,
				IdempotencyKey: "accepted-ban", RequestFingerprint: "accepted-ban-proof", Reason: "bounded investigation", DurationMinutes: 1}, now)
			if err != nil {
				t.Fatal(err)
			}
			completeTestProviderOperation(t, store, ban.ID, now)
			if _, err = store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`, state.role, state.status, state.blocked, actor.ID); err != nil {
				t.Fatal(err)
			}
			// A fractional expiry is later than the whole-second candidate. Text
			// comparison would order the '.' before 'Z' and release it too early.
			if err = store.QueueExpiredAdminTemporaryBan(ctx, owner.ID, now.Add(time.Minute).Truncate(time.Second)); err != nil {
				t.Fatal(err)
			}
			active, err := store.ActiveAdminTemporaryBan(ctx, owner.ID)
			if err != nil || active == nil || active.UnbanOperationID != "" {
				t.Fatal("ban expired before exact instant", active, err)
			}
			for range 2 {
				if err = store.QueueExpiredAdminTemporaryBan(ctx, owner.ID, now.Add(time.Minute)); err != nil {
					t.Fatal("committed expiry depended on fresh administrator authority", err)
				}
			}
			active, err = store.ActiveAdminTemporaryBan(ctx, owner.ID)
			if err != nil || active == nil || active.UnbanOperationID == "" {
				t.Fatal("due ban did not queue restoration", active, err)
			}
			op, err := store.ProviderOperationByID(ctx, active.UnbanOperationID)
			if err != nil || op.ActorUserID != actor.ID || op.OwnerUserID != owner.ID || op.Receipt.Kind != providerops.KindAdminTemporaryUnban {
				t.Fatal("expiry lost stored actor/owner provenance", op, err)
			}
			var count int
			if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operations WHERE owner_user_id=$1 AND kind=$2`, owner.ID, providerops.KindAdminTemporaryUnban).Scan(&count); err != nil || count != 1 {
				t.Fatal("expiry queued more than one restoration", count, err)
			}
		})
	}
}

func TestTemporaryBanExpiryRejectsInactiveBeneficiary(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor, owner := createTestAdmin(t, store, 969017), createTestUser(t, store, 969018)
	now := time.Now().UTC()
	ban, err := store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: actor.ID, UserID: owner.ID,
		IdempotencyKey: "lifecycle-ban", RequestFingerprint: "lifecycle-ban-proof", Reason: "lifecycle check", DurationMinutes: 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	completeTestProviderOperation(t, store, ban.ID, now)
	if _, err = store.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, owner.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.QueueExpiredAdminTemporaryBan(ctx, owner.ID, now.Add(2*time.Minute)); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("expiry granted fresh restoration to purging beneficiary", err)
	}
	active, err := store.ActiveAdminTemporaryBan(ctx, owner.ID)
	if err != nil || active == nil || active.UnbanOperationID != "" {
		t.Fatal("refused lifecycle admission retained continuation", active, err)
	}
}

func TestTemporaryBanExpiryRequiresMatchingStoredOperation(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor, owner := createTestAdmin(t, store, 969019), createTestUser(t, store, 969020)
	now := time.Now().UTC()
	ban, err := store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: actor.ID, UserID: owner.ID,
		IdempotencyKey: "proven-ban", RequestFingerprint: "proven-ban-fingerprint", Reason: "provenance check", DurationMinutes: 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	completeTestProviderOperation(t, store, ban.ID, now)
	for _, field := range []string{"actor", "owner", "kind"} {
		if _, err = store.DB().ExecContext(ctx, `UPDATE provider_operations SET actor_user_id=$1,owner_user_id=$2,kind=$3 WHERE id=$4`, actor.ID, owner.ID, providerops.KindAdminTemporaryBan, ban.ID); err != nil {
			t.Fatal(err)
		}
		var query string
		var value any
		switch field {
		case "actor":
			query, value = `UPDATE provider_operations SET actor_user_id=$1 WHERE id=$2`, owner.ID
		case "owner":
			query, value = `UPDATE provider_operations SET owner_user_id=$1 WHERE id=$2`, actor.ID
		case "kind":
			query, value = `UPDATE provider_operations SET kind=$1 WHERE id=$2`, providerops.KindSubscriptionRevoke
		}
		if _, err = store.DB().ExecContext(ctx, query, value, ban.ID); err != nil {
			t.Fatal(err)
		}
		if err = store.QueueExpiredAdminTemporaryBan(ctx, owner.ID, now.Add(2*time.Minute)); !errors.Is(err, ErrConflict) {
			t.Fatal("expiry continued mismatched original operation", field, err)
		}
	}
	var operations int
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operations WHERE kind=$1`, providerops.KindAdminTemporaryUnban).Scan(&operations); err != nil || operations != 0 {
		t.Fatal("mismatched provenance created restoration work", operations, err)
	}
}
