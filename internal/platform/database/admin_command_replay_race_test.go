package database

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestConcurrentManualGrantReplaysOneOriginalResult(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, actor, owner, combo, now := manualGrantFixture(t)
	input := manualGrantInput(actor.ID, owner.ID, combo.ID, now)
	results := make(chan model.Purchase, 8)
	actions := make([]func() error, 8)
	for i := range actions {
		actions[i] = func() error {
			purchase, err := store.CreateAdminEntitlement(ctx, input, now)
			if err == nil {
				results <- purchase
			}
			return err
		}
	}
	for _, err := range runTogether(actions...) {
		if err != nil {
			t.Fatal("concurrent grant rejected accepted intent", err)
		}
	}
	close(results)
	var accepted model.Purchase
	for purchase := range results {
		if accepted.ID == "" {
			accepted = purchase
		} else if !reflect.DeepEqual(accepted, purchase) {
			t.Fatal("concurrent grant changed original result", accepted, purchase)
		}
	}
	var purchases, audits, jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM purchases WHERE user_id=$1),
		(SELECT COUNT(*) FROM audit_events WHERE action='entitlement.create' AND target_id=$2),
		(SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_sync_user' AND payload::jsonb->>'userId'=$1)`,
		owner.ID, accepted.ID).Scan(&purchases, &audits, &jobs); err != nil || purchases != 1 || audits != 1 || jobs != 1 {
		t.Fatal("concurrent grant duplicated durable work", purchases, audits, jobs, err)
	}
	input.RequestFingerprint = strings.Repeat("b", 64)
	if _, err := store.CreateAdminEntitlement(ctx, input, now); !errors.Is(err, ErrConflict) {
		t.Fatal("changed grant intent replayed", err)
	}
}

func TestConcurrentAdminHourlyDisableReplaysOneReceipt(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor, owner := createTestAdmin(t, store, 89323), createTestUser(t, store, 89324)
	now := time.Now().UTC()
	input := providerops.CreateInput{ActorUserID: actor.ID, OwnerUserID: owner.ID, Kind: "xnode_hourly_disable",
		IdempotencyKey: "0:concurrent", RequestFingerprint: "concurrent-admin-hourly-stop", SealedTarget: "sealed-admin-target",
		Items: []providerops.ItemInput{{Key: "hourly", TargetType: "hourly_squad", TargetID: "hourly-group"}}}
	type outcome struct {
		id       string
		replayed bool
	}
	results := make(chan outcome, 8)
	actions := make([]func() error, 8)
	for i := range actions {
		actions[i] = func() error {
			operation, replayed, err := store.CreateAdminHourlyDisableOperation(ctx, input, now)
			if err == nil {
				results <- outcome{operation.Receipt.ID, replayed}
			}
			return err
		}
	}
	for _, err := range runTogether(actions...) {
		if err != nil {
			t.Fatal("concurrent hourly stop rejected accepted intent", err)
		}
	}
	close(results)
	var accepted string
	fresh := 0
	for result := range results {
		if accepted == "" {
			accepted = result.id
		} else if accepted != result.id {
			t.Fatal("concurrent hourly stop changed receipt", accepted, result.id)
		}
		if !result.replayed {
			fresh++
		}
	}
	var operations, jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM provider_operations WHERE actor_user_id=$1),
		(SELECT COUNT(*) FROM outbox_jobs WHERE kind='provider_operation' AND payload::jsonb->>'operationId'=$2)`,
		actor.ID, accepted).Scan(&operations, &jobs); err != nil || operations != 1 || jobs != 1 || fresh != 1 {
		t.Fatal("concurrent hourly stop duplicated durable work", operations, jobs, fresh, err)
	}
	input.RequestFingerprint = "changed-admin-hourly-stop"
	if _, replayed, err := store.CreateAdminHourlyDisableOperation(ctx, input, now); !errors.Is(err, ErrConflict) || replayed {
		t.Fatal("changed hourly intent replayed", replayed, err)
	}
}
