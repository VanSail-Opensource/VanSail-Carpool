package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNativeMemberProvisioningRejectsOldGenerationBeforeCreatingCredentials(t *testing.T) {
	for _, retained := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "cleared"}[retained], func(t *testing.T) {
			t.Parallel()
			store, ctx := newTestStore(t), context.Background()
			user := createTestUser(t, store, 89311)
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_generation=1,onboarding_state='complete' WHERE id=$1`, user.ID); err != nil {
				t.Fatal(err)
			}
			expected := xnode.Identity{UserID: user.ID}
			if retained {
				var err error
				expected, err = store.SaveXNodeIdentity(ctx, xnode.Identity{UserID: user.ID, AccessDisabled: true})
				if err != nil {
					t.Fatal(err)
				}
			}
			next := expected
			next.CredentialsCipher, next.SubscriptionCipher, next.SubscriptionHash = "new-credentials", "new-subscription", make([]byte, 32)
			if _, err := store.SaveXNodeMemberIdentity(ctx, expected, next, 0); !errors.Is(err, ErrAccountUnavailable) {
				t.Fatal("old job provisioned revived account", err)
			}
			actual, err := store.XNodeIdentityByUser(ctx, user.ID)
			if retained {
				if err != nil || actual.CredentialsCipher != "" || !actual.AccessDisabled {
					t.Fatal(actual, err)
				}
			} else if !errors.Is(err, xnode.ErrNotFound) {
				t.Fatal("rejected provision left credentials", actual, err)
			}
			actual, err = store.SaveXNodeMemberIdentity(ctx, expected, next, 1)
			if err != nil || actual.CredentialsCipher != next.CredentialsCipher || actual.AccessDisabled {
				t.Fatal("current provisioning failed", actual, err)
			}
		})
	}
}

func TestNativeMemberCredentialRepairRetainsConcurrentPointerAndRestriction(t *testing.T) {
	t.Parallel()
	f, ctx := newPurchasePeriodRestoreFixture(t, "open"), context.Background()
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=$1`, f.identity.UserID); err != nil {
		t.Fatal(err)
	}
	next := f.identity
	next.SubscriptionCipher, next.SubscriptionHash = "repaired-token", make([]byte, 32)
	newPeriod := f.period
	newPeriod.ID = "concurrent-period"
	if err := f.store.SaveXNodePeriod(ctx, newPeriod); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET access_disabled=1 WHERE proxy_user_id=$1`, f.identity.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	actual, err := f.store.SaveXNodeMemberIdentity(ctx, f.identity, next, 0)
	if err != nil || actual.CurrentPeriodID != newPeriod.ID || !actual.AccessDisabled || actual.CredentialsCipher != f.identity.CredentialsCipher {
		t.Fatal("repair replaced unrelated current authority", actual, err)
	}
	replay, err := f.store.SaveXNodeMemberIdentity(ctx, f.identity, next, 0)
	if err != nil || replay.CurrentPeriodID != newPeriod.ID || !replay.AccessDisabled || !sameProvisioningMaterial(replay, actual) {
		t.Fatal("provision replay changed exact material", replay, err)
	}
	competing := next
	competing.SubscriptionCipher = "stale-replacement"
	if _, err = f.store.SaveXNodeMemberIdentity(ctx, f.identity, competing, 0); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("stale credential proposal overwrote accepted repair", err)
	}
}

func TestNativeMemberRejoinPreservesActiveBanOnClearedIdentity(t *testing.T) {
	for _, retained := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "cleared"}[retained], func(t *testing.T) {
			t.Parallel()
			store, ctx := newTestStore(t), context.Background()
			user := createTestUser(t, store, 89312)
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin',account_generation=1 WHERE id=$1`, user.ID); err != nil {
				t.Fatal(err)
			}
			expected := xnode.Identity{UserID: user.ID}
			if retained {
				var err error
				expected, err = store.SaveXNodeIdentity(ctx, xnode.Identity{UserID: user.ID, AccessDisabled: true})
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: user.ID, UserID: user.ID, IdempotencyKey: "rejoin-ban", RequestFingerprint: "rejoin-ban-generation-proof", DurationMinutes: 10, Reason: "fixture"}, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			next := expected
			next.CredentialsCipher, next.SubscriptionCipher, next.SubscriptionHash = "current-generation", "current-token", make([]byte, 32)
			actual, err := store.SaveXNodeMemberIdentity(ctx, expected, next, 1)
			if err != nil || !actual.AccessDisabled || actual.CredentialsCipher != next.CredentialsCipher {
				t.Fatal("rejoin provisioning lifted queued ban", actual, err)
			}
		})
	}
}
