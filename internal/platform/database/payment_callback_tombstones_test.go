package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPaymentCallbackTombstoneAndChargeProofSurviveMaintenance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 29101)
	settledAt := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	order := createTestPaymentOrder(t, store, user.ID, "ezpay", 500, settledAt)
	if _, applied, err := store.SettlePayment(ctx, "ezpay", "callback-1", "payload-1",
		order.ID, "trade-1", "charge-1", settledAt); err != nil || !applied {
		t.Fatalf("SettlePayment() = (applied %t, error %v)", applied, err)
	}

	cleanupAt := settledAt.Add(9 * 24 * time.Hour)
	if _, err := store.CompactAndPrune(ctx, cleanupAt.Add(-7*24*time.Hour), cleanupAt.Add(-24*time.Hour), cleanupAt); err != nil {
		t.Fatalf("CompactAndPrune(): %v", err)
	}
	retained, err := store.PaymentOrderByID(ctx, order.ID)
	if err != nil || retained.Status != "paid" || retained.ProviderChargeID == nil || *retained.ProviderChargeID != "charge-1" || retained.TXBMinor != 500 {
		t.Fatalf("maintenance discarded authenticated charge proof: %+v,%v", retained, err)
	}
	replayed, err := store.PaymentCallbackReplay(ctx, "ezpay", "callback-1", order.ID)
	if err != nil || !replayed {
		t.Fatalf("PaymentCallbackReplay() = (%t, %v), want (true, nil)", replayed, err)
	}
	if _, err := store.PaymentCallbackReplay(ctx, "ezpay", "callback-1", "different-order"); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-order replay error = %v, want ErrConflict", err)
	}
	assertRowCount(t, store, "payment_callback_tombstones", 1)
	// Older installations may already have compacted a charge. Its independent
	// callback tombstone must still prevent replay after that historical loss.
	if _, err := store.DB().ExecContext(ctx, `DELETE FROM payment_orders WHERE id=$1`, order.ID); err != nil {
		t.Fatal(err)
	}
	if replayed, err := store.PaymentCallbackReplay(ctx, "ezpay", "callback-1", order.ID); err != nil || !replayed {
		t.Fatalf("historical compacted callback replay = %t,%v", replayed, err)
	}
}
