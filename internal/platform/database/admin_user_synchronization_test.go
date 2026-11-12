package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestAdminNativeSynchronizationMatchesBoundedListAndDetail(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	for index, test := range []struct {
		name              string
		identity          bool
		applied           bool
		clearCredentials  bool
		clearSubscription bool
		legacy            string
		accountStatus     string
		onboardingState   string
		recoveryReason    string
		disabled          bool
		want              string
	}{
		{name: "legacy link only", legacy: "legacy-remote", want: "not_provisioned"},
		{name: "native pending", identity: true, want: "pending"},
		{name: "native applied", identity: true, applied: true, want: "synchronized"},
		{name: "stale legacy link", identity: true, applied: true, legacy: "wrong-remote", want: "synchronized"},
		{name: "recovery failure", identity: true, applied: true, recoveryReason: "native repair failed", want: "failed"},
		{name: "cleared credentials", identity: true, applied: true, clearCredentials: true, legacy: "old-remote", want: "not_provisioned"},
		{name: "cleared subscription", identity: true, applied: true, clearSubscription: true, want: "not_provisioned"},
		{name: "purged owner", identity: true, applied: true, accountStatus: "purged", want: "not_provisioned"},
		{name: "incomplete owner", identity: true, applied: true, onboardingState: "agreement", want: "not_provisioned"},
		{name: "restricted deployed identity", identity: true, applied: true, disabled: true, want: "synchronized"},
	} {
		t.Run(test.name, func(t *testing.T) {
			user := createTestUser(t, store, 9_200_001+int64(index))
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=$1`, user.ID); err != nil {
				t.Fatal(err)
			}
			var nativeID int64
			if test.identity {
				hash := sha256.Sum256([]byte(user.ID))
				identity, err := store.SaveXNodeIdentity(ctx, xnode.Identity{
					UserID: user.ID, CredentialsCipher: "present-credentials",
					SubscriptionHash: hash[:], SubscriptionCipher: "present-subscription", AccessDisabled: test.disabled,
				})
				if err != nil {
					t.Fatal(err)
				}
				nativeID = identity.ProxyUserID
				var applied any
				if test.applied {
					applied = stamp(time.Now().UTC())
				}
				credentials, subscription := identity.CredentialsCipher, identity.SubscriptionCipher
				if test.clearCredentials {
					credentials = ""
				}
				if test.clearSubscription {
					subscription = ""
				}
				// Seed retained deployment evidence directly, including the
				// deliberately incomplete material that must never qualify.
				if _, err := store.DB().ExecContext(ctx, `UPDATE xnode_identities SET first_synced_at=$1,credentials_cipher=$2,subscription_cipher=$3 WHERE proxy_user_id=$4`,
					applied, credentials, subscription, nativeID); err != nil {
					t.Fatal(err)
				}
			}
			status, onboarding := test.accountStatus, test.onboardingState
			if status == "" {
				status = "active"
			}
			if onboarding == "" {
				onboarding = "complete"
			}
			// Complete onboarding while the owner is active; the lifecycle trigger
			// correctly rejects setting completion on an already unavailable owner.
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state=$1 WHERE id=$2`, onboarding, user.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status=$1,recovery_reason=$2,proxy_user_id=$3 WHERE id=$4`,
				status, test.recoveryReason, nullText(test.legacy), user.ID); err != nil {
				t.Fatal(err)
			}
			detail, err := store.AdminUserSynchronization(ctx, user.ID)
			if err != nil || detail.Status != test.want {
				t.Fatalf("native synchronization = %+v, %v", detail, err)
			}
			if detail.RemoteUserID != nil && *detail.RemoteUserID != strconv.FormatInt(nativeID, 10) {
				t.Fatalf("legacy or unrelated identity projected: %+v", detail)
			}
			if (test.want == "pending" || test.want == "synchronized") && detail.RemoteUserID == nil {
				t.Fatalf("current native identity missing: %+v", detail)
			}
			if test.want == "not_provisioned" && detail.RemoteUserID != nil {
				t.Fatalf("unusable identity projected: %+v", detail)
			}
			if test.recoveryReason != "" && (detail.LastError == nil || *detail.LastError != test.recoveryReason) {
				t.Fatalf("recovery evidence lost: %+v", detail)
			}
			items, _, err := store.ListAdminUsersPage(ctx, "", AdminUserSearchFilter{Search: user.ID}, 25)
			if err != nil {
				t.Fatal(err)
			}
			if status != "active" {
				if len(items) != 0 {
					t.Fatalf("inactive owner entered active list: %+v", items)
				}
			} else if len(items) != 1 || !reflect.DeepEqual(items[0].Synchronization, detail) {
				t.Fatalf("list/detail synchronization differs: %+v vs %+v", items, detail)
			}
		})
	}
	if _, err := store.AdminUserSynchronization(ctx, "missing-owner"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing synchronization owner = %v", err)
	}
}
