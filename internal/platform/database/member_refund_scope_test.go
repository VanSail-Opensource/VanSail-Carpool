package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMemberRefundAdmissionRequiresExactOwnerPurchaseItem(t *testing.T) {
	for _, scenario := range []string{"actor", "key", "type", "target"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
			owner := createTestUser(t, store, 90401)
			other := createTestUser(t, store, 90402)
			combo := saveTestCombo(t, store, "Refund scope", 100, 30)
			if _, err := store.AdjustBalance(ctx, owner.ID, 500, "refund-scope-seed", "fixture", now); err != nil {
				t.Fatal(err)
			}
			purchase := createMemberOperationPurchase(t, store, owner.ID, combo.ID, "refund-scope", now)
			if err := store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now); err != nil {
				t.Fatal(err)
			}
			input := memberOperationInput(owner.ID, purchase.ID, purchaseops.OperationRefundKind, "refund-scope")
			switch scenario {
			case "actor":
				input.ActorUserID = other.ID
			case "key":
				input.Items[0].Key = "unrelated"
			case "type":
				input.Items[0].TargetType = "user"
			case "target":
				input.Items[0].TargetID = "unrelated"
			}
			if _, _, err := store.BeginMemberRefund(ctx, input, purchase.ID, now.Add(time.Hour)); !errors.Is(err, ErrConflict) {
				t.Fatal("malformed refund command admitted", err)
			}
			var count int
			if err := store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM provider_operations").Scan(&count); err != nil || count != 0 {
				t.Fatal("rejected admission retained a command", count, err)
			}
			if got := adminWorkflowBalance(t, store, owner.ID); got != 400 {
				t.Fatal("rejected admission changed the wallet", got)
			}
		})
	}
}

func TestMemberRefundFinalizationRequiresExactAttemptProof(t *testing.T) {
	for _, scenario := range []string{"kind", "actor", "key", "type", "target", "status", "attempt", "extra item"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, store, purchaseID, operation, now := refundNoticeFixture(t)
			var query string
			args := []any{operation.Receipt.ID}
			switch scenario {
			case "kind":
				query = "UPDATE provider_operations SET kind='member_early_activation' WHERE id=$1"
			case "actor":
				other := createTestUser(t, store, 90403)
				query, args = "UPDATE provider_operations SET actor_user_id=$2 WHERE id=$1", []any{operation.Receipt.ID, other.ID}
			case "key":
				query = "UPDATE provider_operation_items SET item_key='unrelated' WHERE operation_id=$1"
			case "type":
				query = "UPDATE provider_operation_items SET target_type='user' WHERE operation_id=$1"
			case "target":
				query = "UPDATE provider_operation_items SET target_id='unrelated' WHERE operation_id=$1"
			case "status":
				query = "UPDATE provider_operation_items SET status='queued' WHERE operation_id=$1"
			case "attempt":
				query = "UPDATE provider_operation_items SET attempt_started_at=NULL WHERE operation_id=$1"
			case "extra item":
				query = "INSERT INTO provider_operation_items(operation_id,item_key,target_type,target_id,status,created_at,updated_at) SELECT operation_id,'extra',target_type,target_id,'queued',created_at,updated_at FROM provider_operation_items WHERE operation_id=$1"
			}
			if _, err := store.DB().ExecContext(ctx, query, args...); err != nil {
				t.Fatal(err)
			}
			if _, err := store.FinalizeMemberRefund(ctx, operation.Receipt.ID, purchaseID, now); !errors.Is(err, ErrConflict) {
				t.Fatal("refund committed without exact attempted command", err)
			}
			assertRejectedRefundUnchanged(t, store, operation, purchaseID)
		})
	}
}

func TestMemberRefundReadFailuresRemainStorageErrors(t *testing.T) {
	for _, table := range []string{"provider_operations", "combos"} {
		t.Run(table, func(t *testing.T) {
			ctx, store, purchaseID, operation, now := refundNoticeFixture(t)
			temporary := "unavailable_" + table
			if _, err := store.DB().ExecContext(ctx, "ALTER TABLE "+table+" RENAME TO "+temporary); err != nil {
				t.Fatal(err)
			}
			_, failure := store.FinalizeMemberRefund(ctx, operation.Receipt.ID, purchaseID, now)
			if _, err := store.DB().ExecContext(ctx, "ALTER TABLE "+temporary+" RENAME TO "+table); err != nil {
				t.Fatal(err)
			}
			var sqlFailure *pgconn.PgError
			if !errors.As(failure, &sqlFailure) || sqlFailure.Code != "42P01" ||
				errors.Is(failure, ErrConflict) || errors.Is(failure, purchaseops.ErrIneligible) {
				t.Fatal("storage failure was converted into a business refusal", failure)
			}
			assertRejectedRefundUnchanged(t, store, operation, purchaseID)
		})
	}
}

func assertRejectedRefundUnchanged(t *testing.T, store *Store, operation providerops.Operation, purchaseID string) {
	t.Helper()
	ctx := context.Background()
	if got := adminWorkflowBalance(t, store, operation.OwnerUserID); got != 400 {
		t.Fatal("rejected refund changed the wallet", got)
	}
	var credits int
	if err := store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM ledger_entries WHERE kind='member_refund_credit'").Scan(&credits); err != nil || credits != 0 {
		t.Fatal("rejected refund retained a credit", credits, err)
	}
	purchase, err := store.PurchaseByID(ctx, purchaseID)
	if err != nil || purchase.Status != "active" {
		t.Fatal("rejected refund changed the purchase", purchase.Status, err)
	}
	current, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
	if err != nil || current.Receipt.Status != "processing" {
		t.Fatal("rejected refund completed the receipt", current.Receipt.Status, err)
	}
}
