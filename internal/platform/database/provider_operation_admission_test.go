package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestFreshAdminOperationAdmissionAndImmutableReplay(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor, owner := createTestAdmin(t, store, 969011), createTestUser(t, store, 969012)
	now := time.Date(2026, 11, 13, 21, 0, 0, 0, time.UTC)
	kinds := []string{providerops.KindAdminEntitlementEdit, providerops.KindAdminEntitlementRefund,
		providerops.KindAdminComboReplacement, providerops.KindAdminBulkExtension,
		providerops.KindAdminTemporaryBan, providerops.KindAdminTemporaryUnban,
		providerops.KindAdminMaintenance, providerops.KindPaymentRefund, providerops.KindOutboxRetry}
	accepted := map[string]string{}
	input := func(kind, key string) providerops.CreateInput {
		return providerops.CreateInput{ActorUserID: actor.ID, OwnerUserID: owner.ID, Kind: kind, IdempotencyKey: key, RequestFingerprint: "admin-admission-proof"}
	}
	for _, kind := range kinds {
		op, replayed, err := store.CreateProviderOperation(ctx, input(kind, "accepted"), now)
		if err != nil || replayed {
			t.Fatal("initial administrator admission", kind, replayed, err)
		}
		accepted[kind] = op.Receipt.ID
	}
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"demoted", "user", "active", 0}, {"purging", "admin", "purging", 0}, {"blocked", "admin", "active", 1},
	} {
		if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`, state.role, state.status, state.blocked, actor.ID); err != nil {
			t.Fatal(err)
		}
		for _, kind := range kinds {
			if op, replayed, err := store.CreateProviderOperation(ctx, input(kind, "fresh-"+state.name), now); !errors.Is(err, ErrConflict) || replayed || op.Receipt.ID != "" {
				t.Fatalf("%s admitted fresh %s: %+v %t %v", state.name, kind, op, replayed, err)
			}
			op, replayed, err := store.CreateProviderOperation(ctx, input(kind, "accepted"), now.Add(time.Minute))
			if err != nil || !replayed || op.Receipt.ID != accepted[kind] {
				t.Fatal("accepted replay depended on current administrator authority", state.name, kind, replayed, err)
			}
		}
		if _, err := store.CreateAdminTemporaryUnban(ctx, actor.ID, owner.ID, "expiry:forged-command", "expiry:forged-command", "not a continuation", now); !errors.Is(err, ErrConflict) {
			t.Fatal("public key prefix bypassed fresh unban authority", err)
		}
	}
	var operations, jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM provider_operations),(SELECT COUNT(*) FROM outbox_jobs WHERE kind='provider_operation')`).Scan(&operations, &jobs); err != nil || operations != len(kinds) || jobs != len(kinds) {
		t.Fatal("rejected or replayed commands created work", operations, jobs, err)
	}
	member := providerops.CreateInput{ActorUserID: owner.ID, OwnerUserID: owner.ID, Kind: providerops.KindSubscriptionRevoke, IdempotencyKey: "member-command", RequestFingerprint: "member-command-proof"}
	if _, replayed, err := store.CreateProviderOperation(ctx, member, now); err != nil || replayed {
		t.Fatal("admin-only fence denied a member-domain command", replayed, err)
	}
}

func TestRejectedAdminRefundLeavesMoneyAndEntitlementUntouched(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	actor := createTestAdmin(t, store, 969013)
	combo := saveTestCombo(t, store, "Current refund authority", 100, 30)
	owner, purchase := createAdminWorkflowPurchase(t, store, 969014, combo, now)
	before := adminWorkflowBalance(t, store, owner.ID)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RefundAdminEntitlement(ctx, AdminEntitlementRefundInput{ActorUserID: actor.ID, UserID: owner.ID, PurchaseID: purchase.ID,
		IdempotencyKey: "denied-refund", RequestFingerprint: "denied-refund-proof", Reason: "stale administrator", AmountTXBMinor: 100}, now); !errors.Is(err, ErrConflict) {
		t.Fatal("demoted administrator refunded purchase", err)
	}
	current, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil || current.Status != purchase.Status || !current.UpdatedAt.Equal(purchase.UpdatedAt) || adminWorkflowBalance(t, store, owner.ID) != before || adminWorkflowLedgerCount(t, store, owner.ID, "admin_entitlement_refund") != 0 {
		t.Fatal("refused refund left financial state", current, err)
	}
	var operations, audits int
	if err = store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM provider_operations WHERE actor_user_id=$1),(SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1)`, actor.ID).Scan(&operations, &audits); err != nil || operations != 0 || audits != 0 {
		t.Fatal("refused refund left operation/audit evidence", operations, audits, err)
	}
}
