package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestMaintenanceKeepsPaidRefundedAndUncertainPaymentEvidence(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 497701)
	now := time.Now().UTC()
	old := now.Add(-10 * 24 * time.Hour)
	create := func() model.PaymentOrder {
		t.Helper()
		order, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{UserID: owner.ID, Provider: "stars",
			Status: "pending", TXBMinor: 100, PayableAmount: "1", PayableCurrency: "XTR", RateSnapshot: "1",
			ExpiresAt: now.Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		return order
	}
	paid, refunded, failed, uncertain := create(), create(), create(), create()
	// Backdate the complete fixture after creation so the fixture constructor
	// does not expire the earlier pending orders while seeding the later ones.
	if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET created_at=$1,updated_at=$1,expires_at=$2 WHERE user_id=$3`, stamp(old), stamp(old.Add(time.Hour)), owner.ID); err != nil {
		t.Fatal(err)
	}
	// A cancelled order can settle later; its old cancellation marker must not
	// make the now-paid charge eligible for deletion through an OR condition.
	if _, _, err := store.CancelPaymentOrder(ctx, paid.ID, owner.ID, "user", old); err != nil {
		t.Fatal(err)
	}
	for _, order := range []model.PaymentOrder{paid, refunded} {
		if _, applied, err := store.SettlePayment(ctx, "stars", order.ID, "proof-"+order.ID, order.ID, "trade-"+order.ID, "charge-"+order.ID, old.Add(time.Minute)); err != nil || !applied {
			t.Fatal("settle retained charge", applied, err)
		}
	}
	if _, err := store.RefundPayment(ctx, nil, refunded.ID, "provider refund", old.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	for _, order := range []model.PaymentOrder{failed, uncertain} {
		if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET status='failed',updated_at=$1 WHERE id=$2`, stamp(old), order.ID); err != nil {
			t.Fatal(err)
		}
	}
	op, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: owner.ID, OwnerUserID: owner.ID,
		Kind: "payment_create", IdempotencyKey: "uncertain-payment-retention", RequestFingerprint: "uncertain-payment-retention-proof",
		Items: []providerops.ItemInput{{Key: "order", TargetType: "payment_order", TargetID: uncertain.ID}}}, old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE provider_operations SET status='pending_review',attempts=1,attempt_started_at=$1 WHERE id=$2`, stamp(old), op.Receipt.ID); err != nil {
		t.Fatal(err)
	}
	for iteration := range 2 {
		counts, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
		if err != nil || counts["payment_orders"] != int64(1-iteration) || counts["refunds"] != 0 {
			t.Fatal("unexpected payment evidence compaction", counts, err)
		}
		for _, order := range []model.PaymentOrder{paid, refunded, uncertain} {
			if _, err := store.PaymentOrderByID(ctx, order.ID); err != nil {
				t.Fatal("authoritative payment detail was deleted", err)
			}
		}
	}
	if _, err := store.PaymentOrderByID(ctx, failed.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("unpaid settled failure no longer compacts", err)
	}
	for range 2 {
		if _, err := store.RefundPayment(ctx, nil, paid.ID, "delayed provider refund", now); err != nil {
			t.Fatal("late refund could not use retained charge", err)
		}
	}
	balance, err := store.Balance(ctx, owner.ID)
	if err != nil || balance.Minor != "0" {
		t.Fatal("delayed refund was lost or duplicated", balance, err)
	}
	var refunds, reversals int
	if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM refunds WHERE payment_order_id IN ($1,$2)`, paid.ID, refunded.ID).Scan(&refunds); err != nil || refunds != 2 {
		t.Fatal("refund evidence not retained exactly once", refunds, err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM ledger_entries WHERE kind='payment_reversal' AND reference_id=$1`, paid.ID).Scan(&reversals); err != nil || reversals != 1 {
		t.Fatal("late reversal repeated", reversals, err)
	}
}

func TestBulkExtensionReplaySurvivesMaintenanceWithoutRepeatingEntitlement(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	old := now.Add(-10 * 24 * time.Hour)
	combo := saveTestCombo(t, store, "retained-bulk-replay", 100, 30)
	_, purchase := createAdminWorkflowPurchase(t, store, 497702, combo, old)
	actor := createTestAdmin(t, store, 497703)
	input := AdminBulkExtensionInput{ActorUserID: actor.ID, IdempotencyKey: "retained-bulk-extension",
		RequestFingerprint: "retained-bulk-extension-proof", Reason: "service credit", DurationMinutes: 60,
		Filter: AdminBulkExtensionFilter{ComboIDs: []string{combo.ID}}}
	receipt, err := store.CreateAdminBulkExtension(ctx, input, old)
	if err != nil {
		t.Fatal(err)
	}
	completeTestProviderOperation(t, store, receipt.ID, old.Add(time.Minute))
	for range 2 {
		if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
			t.Fatal(err)
		}
		replay, err := store.CreateAdminBulkExtension(ctx, input, now)
		if err != nil || replay.ID != receipt.ID || replay.Status != "succeeded" {
			t.Fatal("maintenance discarded the original command receipt", replay, err)
		}
		current, err := store.PurchaseByID(ctx, purchase.ID)
		if err != nil || !current.ValidUntil.Equal(purchase.ValidUntil.Add(time.Hour)) {
			t.Fatal("replayed extension applied a second time", current, err)
		}
	}
	input.RequestFingerprint = "different-request-on-retained-key"
	if _, err := store.CreateAdminBulkExtension(ctx, input, now); !errors.Is(err, ErrConflict) {
		t.Fatal("retained command key accepted a different intent", err)
	}
}
