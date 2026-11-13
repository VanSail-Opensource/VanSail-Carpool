package database

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestPricingNetRevenueIncludesEveryPurchaseRefundLane(t *testing.T) {
	t.Parallel()
	for _, lane := range []string{"admin_refund", "admin_cancel", "queued_cancel", "member_refund"} {
		t.Run(lane, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			now := time.Date(2026, 11, 13, 13, 0, 0, 0, time.UTC)
			actor := createTestAdmin(t, store, 91901)
			combo := saveTestCombo(t, store, "net-revenue", 1_700, 30)
			user, purchase := createAdminWorkflowPurchase(t, store, 91902, combo, now)
			grossReceipts := purchase.PriceTXBMinor
			if lane == "queued_cancel" {
				var err error
				purchase, err = store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID,
					IdempotencyKey: "queued-net-revenue"}, now.Add(time.Minute))
				if err != nil {
					t.Fatal(err)
				}
				grossReceipts += purchase.PriceTXBMinor
			}
			credit := refundHistoryPurchase(t, store, actor.ID, purchase, lane, now.Add(2*time.Minute))
			summary, err := store.PricingSummary(ctx, now.Add(5*time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			if got, want := summary["netRevenueMinor"], strconv.FormatInt(grossReceipts-credit, 10); got != want {
				t.Fatalf("%s net revenue = %v; want %s after %d credit", lane, got, want, credit)
			}
			// Product gross receipts remain a separate before-refund statistic.
			actual, status, err := store.pricingActual(ctx, "product", "combo:"+combo.ID, "gross_revenue_minor", now.Add(-time.Hour), now.Add(time.Hour))
			if err != nil || status != "COMPLETE" || actual == nil || *actual != float64(grossReceipts) {
				t.Fatalf("gross receipt history changed with refund: %v, %s, %v", actual, status, err)
			}
		})
	}
}
