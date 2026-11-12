package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestAdminOperationReplaySurvivesExpiredTargets(t *testing.T) {
	for _, kind := range []string{"bulk extension", "combo replacement"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
			combo := saveTestCombo(t, store, "Original", 100, 30)
			target := saveTestCombo(t, store, "Replacement", 200, 30)
			user, purchase := createAdminWorkflowPurchase(t, store, 42901, combo, now)
			actor := createTestUser(t, store, 42902)
			command := func(fingerprint string) (model.OperationReceipt, error) {
				if kind == "bulk extension" {
					return store.CreateAdminBulkExtension(ctx, AdminBulkExtensionInput{
						ActorUserID: actor.ID, IdempotencyKey: "recover", RequestFingerprint: fingerprint,
						Reason: "service credit", Filter: AdminBulkExtensionFilter{ComboIDs: []string{combo.ID}},
						DurationMinutes: 60,
					}, now)
				}
				return store.ReplaceAdminCombo(ctx, AdminComboReplacementInput{
					ActorUserID: actor.ID, UserID: user.ID, ComboID: target.ID,
					IdempotencyKey: "recover", RequestFingerprint: fingerprint, Reason: "service replacement",
				}, now)
			}
			first, err := command("original-request-fingerprint")
			if err != nil {
				t.Fatal(err)
			}
			after, err := store.PurchaseByID(ctx, purchase.ID)
			if err != nil {
				t.Fatal(err)
			}
			// Simulate recovery after the original term has left live admission.
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='expired' WHERE id=$1`, purchase.ID); err != nil {
				t.Fatal(err)
			}
			replayed, err := command("original-request-fingerprint")
			if err != nil || replayed.ID != first.ID {
				t.Fatalf("recovery after expiry = %+v, %v", replayed, err)
			}
			if _, err := command("different-request-fingerprint"); !errors.Is(err, ErrConflict) {
				t.Fatalf("changed intent after expiry = %v", err)
			}
			current, err := store.PurchaseByID(ctx, purchase.ID)
			if err != nil || current.Status != "expired" || !current.ValidUntil.Equal(after.ValidUntil) || current.ComboID != after.ComboID {
				t.Fatalf("recovery repeated or changed mutation: %+v, %v", current, err)
			}
			var operations, items, audits int
			if err := store.DB().QueryRowContext(ctx, `SELECT
				(SELECT COUNT(*) FROM provider_operations WHERE actor_user_id=$1),
				(SELECT COUNT(*) FROM provider_operation_items WHERE operation_id=$2),
				(SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1)`, actor.ID, first.ID).
				Scan(&operations, &items, &audits); err != nil || operations != 1 || items != 1 || audits != 1 {
				t.Fatalf("recovery evidence: operations=%d items=%d audits=%d err=%v", operations, items, audits, err)
			}
		})
	}
}
