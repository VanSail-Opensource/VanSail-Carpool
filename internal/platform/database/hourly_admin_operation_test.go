package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestAdminHourlyDisableAdmissionFencesCurrentActorAndKeepsAcceptedReplay(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor, owner := createTestAdmin(t, store, 89321), createTestUser(t, store, 89322)
	input := providerops.CreateInput{ActorUserID: actor.ID, OwnerUserID: owner.ID, Kind: "xnode_hourly_disable",
		IdempotencyKey: "0:accepted", RequestFingerprint: "admin-hourly-stop", SealedTarget: "sealed-admin-target",
		Items: []providerops.ItemInput{{Key: "hourly", TargetType: "hourly_squad", TargetID: "hourly-group"}}}
	accepted, replayed, err := store.CreateAdminHourlyDisableOperation(ctx, input, time.Now())
	if err != nil || replayed {
		t.Fatal(accepted, replayed, err)
	}
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"demoted", "user", "active", 0}, {"purging", "admin", "purging", 0}, {"blocked", "admin", "active", 1},
	} {
		if _, err = store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`, state.role, state.status, state.blocked, actor.ID); err != nil {
			t.Fatal(err)
		}
		fresh := input
		fresh.IdempotencyKey = "0:fresh-" + state.name
		if _, replayed, err = store.CreateAdminHourlyDisableOperation(ctx, fresh, time.Now()); !errors.Is(err, ErrConflict) || replayed {
			t.Fatal("ineligible actor admitted fresh intent", state.name, replayed, err)
		}
		replay, replayed, err := store.CreateAdminHourlyDisableOperation(ctx, input, time.Now())
		if err != nil || !replayed || replay.Receipt.ID != accepted.Receipt.ID {
			t.Fatal("accepted intent depended on current role", state.name, replay, replayed, err)
		}
	}
	var operations, jobs int
	if err = store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM provider_operations WHERE actor_user_id=$1),
		(SELECT COUNT(*) FROM outbox_jobs WHERE kind='provider_operation' AND payload::jsonb->>'operationId'=$2)`, actor.ID, accepted.Receipt.ID).Scan(&operations, &jobs); err != nil || operations != 1 || jobs != 1 {
		t.Fatal("rejection/replay created more work", operations, jobs, err)
	}
	member := input
	member.ActorUserID, member.IdempotencyKey, member.RequestFingerprint = owner.ID, "0:member", "member-hourly-stop"
	if _, _, err = store.CreateProviderOperation(ctx, member, time.Now()); err != nil {
		t.Fatal("member lane required administrator authority", err)
	}
}
