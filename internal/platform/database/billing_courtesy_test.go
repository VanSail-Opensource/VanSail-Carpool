package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCourtesyCreditPreservesTerminalPaymentState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	member := createTestUser(t, store, 10_007)
	admin := createTestUser(t, store, 10_008)
	now := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.UTC)
	expired := createTestPaymentOrder(t, store, member.ID, "ezpay", 2_500, now)
	if err := store.ExpirePaymentOrder(ctx, expired.ID, "ezpay", now); err != nil {
		t.Fatalf("ExpirePaymentOrder(): %v", err)
	}
	credit, err := store.CourtesyCreditPayment(ctx, admin.ID, expired.ID, "provider support confirmed failure", now)
	if err != nil || credit.TXB.Minor != "2500" || credit.Replayed {
		t.Fatalf("CourtesyCreditPayment() = (%+v, %v)", credit, err)
	}
	replayed, err := store.CourtesyCreditPayment(ctx, admin.ID, expired.ID, "duplicate request", now.Add(time.Minute))
	if err != nil || !replayed.Replayed || replayed.ID != credit.ID {
		t.Fatalf("CourtesyCreditPayment(replay) = (%+v, %v)", replayed, err)
	}
	order, err := store.PaymentOrderByID(ctx, expired.ID)
	if err != nil || order.Status != "expired" || order.PaidAt != nil {
		t.Fatalf("PaymentOrderByID() = (%+v, %v), want unchanged expired order", order, err)
	}
	if _, _, err := store.SettlePayment(ctx, "ezpay", "late-event", "late-payload", expired.ID, "late-trade", "", now.Add(2*time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("SettlePayment(courtesy credited) = %v, want conflict", err)
	}
	balance, err := store.Balance(ctx, member.ID)
	if err != nil || balance.Minor != "2500" {
		t.Fatalf("Balance() = (%+v, %v), want 2500", balance, err)
	}
	entries, err := store.ListLedger(ctx, member.ID, 10)
	if err != nil || countLedgerKind(entries, "payment_courtesy_credit") != 1 {
		t.Fatalf("ListLedger() = (%+v, %v), want one courtesy credit", entries, err)
	}
	events, err := store.ListAuditEvents(ctx, 10)
	if err != nil {
		t.Fatalf("ListAuditEvents() = (%+v, %v), want courtesy audit", events, err)
	}
	courtesyAudits := 0
	for _, event := range events {
		if event.Action == "payment.courtesy_credit" && event.TargetID == expired.ID {
			courtesyAudits++
		}
	}
	if courtesyAudits != 1 {
		t.Fatalf("ListAuditEvents() = %+v, want exactly one courtesy audit for %s", events, expired.ID)
	}
	assertNotificationCounts(t, store, 1, 1)

	failed, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{UserID: member.ID, Provider: "ezpay", Status: "creating", TXBMinor: 300,
		PayableAmount: "3.00", PayableCurrency: "CNY", RateSnapshot: "1", ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatalf("CreatePaymentOrder(failed): %v", err)
	}
	if err := store.FailPaymentOrder(ctx, failed.ID); err != nil {
		t.Fatalf("FailPaymentOrder(): %v", err)
	}
	if _, err := store.CourtesyCreditPayment(ctx, admin.ID, failed.ID, "checkout was unavailable", now); err != nil {
		t.Fatalf("CourtesyCreditPayment(failed): %v", err)
	}
}

func TestCompactAndPrunePreservesCourtesyCreditEvidence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	member := createTestUser(t, store, 10_009)
	admin := createTestUser(t, store, 10_010)
	creditedAt := time.Date(2026, time.August, 1, 10, 0, 0, 0, time.UTC)
	order := createTestPaymentOrder(t, store, member.ID, "ezpay", 2_500, creditedAt)
	if err := store.ExpirePaymentOrder(ctx, order.ID, "ezpay", creditedAt); err != nil {
		t.Fatalf("ExpirePaymentOrder(): %v", err)
	}
	if _, err := store.CourtesyCreditPayment(ctx, admin.ID, order.ID, "provider support confirmed failure", creditedAt); err != nil {
		t.Fatalf("CourtesyCreditPayment(): %v", err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET updated_at=? WHERE id=?`, stamp(creditedAt), order.ID); err != nil {
		t.Fatalf("age payment order: %v", err)
	}

	now := creditedAt.Add(8 * 24 * time.Hour)
	counts, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("CompactAndPrune(): %v", err)
	}
	if counts["payment_orders"] != 0 {
		t.Fatalf("pruned payment orders = %d, want 0", counts["payment_orders"])
	}
	if _, err := store.PaymentOrderByID(ctx, order.ID); err != nil {
		t.Fatalf("PaymentOrderByID() = %v, want preserved courtesy-credit evidence", err)
	}
	var credits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM courtesy_credits WHERE payment_order_id=?`, order.ID).Scan(&credits); err != nil {
		t.Fatalf("count courtesy credits: %v", err)
	}
	if credits != 1 {
		t.Fatalf("courtesy credits = %d, want 1", credits)
	}
}
