package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestAdminMutationCommitsOneAuditWithIntentAndJob(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	actor := createTestUser(t, s, 95001)
	now := time.Now().UTC()
	input := adminMutationTestInput(actor.ID, "atomic-maintenance")
	operation, replayed, err := s.CreateAdminMutation(ctx, input, now)
	if err != nil || replayed {
		t.Fatal(operation, replayed, err)
	}
	assertAdminMutationEvidence(t, s, 1)
	var auditActor, target, detail string
	if err := s.db.QueryRowContext(ctx, "SELECT actor_user_id,target_id,detail FROM audit_events WHERE action=$1",
		input.Kind+".queue").Scan(&auditActor, &target, &detail); err != nil {
		t.Fatal(err)
	}
	var recorded map[string]string
	if err := json.Unmarshal([]byte(detail), &recorded); err != nil {
		t.Fatal(err)
	}
	if auditActor != actor.ID || target != "retention" || recorded["operationId"] != operation.Receipt.ID {
		t.Fatal("audit is not bound to the queued command", auditActor, target, detail)
	}
	replay, replayed, err := s.CreateAdminMutation(ctx, input, now.Add(time.Second))
	if err != nil || !replayed || replay.Receipt.ID != operation.Receipt.ID {
		t.Fatal(replay, replayed, err)
	}
	assertAdminMutationEvidence(t, s, 1)
	input.RequestFingerprint = "changed-maintenance-payload"
	if _, _, err := s.CreateAdminMutation(ctx, input, now); !errors.Is(err, ErrConflict) {
		t.Fatal("changed payload reused a receipt", err)
	}
	assertAdminMutationEvidence(t, s, 1)
}

func TestAdminMutationAuditFailureRollsBackQueue(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	actor := createTestUser(t, s, 95002)
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `CREATE FUNCTION reject_admin_queue_audit() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit storage unavailable'; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER reject_admin_queue_audit BEFORE INSERT ON audit_events
		FOR EACH ROW EXECUTE FUNCTION reject_admin_queue_audit()`); err != nil {
		t.Fatal(err)
	}
	input := adminMutationTestInput(actor.ID, "failed-audit")
	if operation, _, err := s.CreateAdminMutation(ctx, input, now); err == nil || operation.Receipt.ID != "" {
		t.Fatal("queued without a durable audit", operation, err)
	}
	assertAdminMutationEvidence(t, s, 0)
	if _, err := s.db.ExecContext(ctx, "DROP TRIGGER reject_admin_queue_audit ON audit_events"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "DROP FUNCTION reject_admin_queue_audit()"); err != nil {
		t.Fatal(err)
	}
	if _, replayed, err := s.CreateAdminMutation(ctx, input, now); err != nil || replayed {
		t.Fatal("failed intent was not rolled back", replayed, err)
	}
	assertAdminMutationEvidence(t, s, 1)
}

func TestAdminRefundQueueBindsThePaymentOwner(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	actor := createTestUser(t, s, 95003)
	owner := createTestUser(t, s, 95004)
	now := time.Now().UTC()
	order := createTestPaymentOrder(t, s, owner.ID, "stars", 100, now)
	input := providerops.CreateInput{
		ActorUserID: actor.ID, OwnerUserID: actor.ID, Kind: providerops.KindPaymentRefund,
		IdempotencyKey: "refund-owner", RequestFingerprint: "refund-owner-fingerprint",
		Items: []providerops.ItemInput{{Key: "target", TargetType: "payment_order", TargetID: order.ID}},
	}
	if _, _, err := s.CreateAdminMutation(ctx, input, now); !errors.Is(err, ErrConflict) {
		t.Fatal("foreign owner accepted", err)
	}
	assertAdminMutationEvidence(t, s, 0)
	input.OwnerUserID = owner.ID
	operation, _, err := s.CreateAdminMutation(ctx, input, now)
	if err != nil || operation.OwnerUserID != owner.ID {
		t.Fatal(operation, err)
	}
	assertAdminMutationEvidence(t, s, 1)
	input.OwnerUserID = actor.ID
	if _, _, err := s.CreateAdminMutation(ctx, input, now); !errors.Is(err, ErrConflict) {
		t.Fatal("replay changed the payment owner", err)
	}
	assertAdminMutationEvidence(t, s, 1)
}

func TestAdminMutationRejectsUnrelatedKindsAndTargets(t *testing.T) {
	for _, change := range []string{"kind", "item", "target", "owner"} {
		t.Run(change, func(t *testing.T) {
			s := newTestStore(t)
			actor := createTestUser(t, s, 95005)
			input := adminMutationTestInput(actor.ID, change)
			switch change {
			case "kind":
				input.Kind = "unrelated"
			case "item":
				input.Items[0].Key = "other"
			case "target":
				input.Items[0].TargetID = "other"
			case "owner":
				input.OwnerUserID = "foreign"
			}
			if _, _, err := s.CreateAdminMutation(context.Background(), input, time.Now()); !errors.Is(err, ErrConflict) {
				t.Fatal("unrelated privileged intent accepted", err)
			}
			assertAdminMutationEvidence(t, s, 0)
		})
	}
}

func adminMutationTestInput(actorID, key string) providerops.CreateInput {
	return providerops.CreateInput{
		ActorUserID: actorID, OwnerUserID: actorID, Kind: providerops.KindAdminMaintenance,
		IdempotencyKey: key, RequestFingerprint: "maintenance-fingerprint",
		Items: []providerops.ItemInput{{Key: "target", TargetType: "maintenance", TargetID: "retention"}},
	}
}

func assertAdminMutationEvidence(t *testing.T, s *Store, want int) {
	t.Helper()
	var operations, items, jobs, audits int
	err := s.db.QueryRowContext(context.Background(), `SELECT
		(SELECT COUNT(*) FROM provider_operations),
		(SELECT COUNT(*) FROM provider_operation_items),
		(SELECT COUNT(*) FROM outbox_jobs WHERE kind='provider_operation'),
		(SELECT COUNT(*) FROM audit_events WHERE action IN ('admin_maintenance.queue','payment_refund.queue','outbox_retry.queue'))`).
		Scan(&operations, &items, &jobs, &audits)
	if err != nil || operations != want || items != want || jobs != want || audits != want {
		t.Fatalf("intent/items/jobs/audits=%d/%d/%d/%d want=%d: %v", operations, items, jobs, audits, want, err)
	}
}
