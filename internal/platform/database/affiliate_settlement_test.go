package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestAffiliateSettlementUsesPreUpgradeTierOnce(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	inviter := createTestUser(t, store, 701)
	if _, accepted, err := store.AcceptAffiliateReferral(ctx, 702, 701, time.Now().UTC()); err != nil || !accepted {
		t.Fatalf("AcceptAffiliateReferral() = %v, %v", accepted, err)
	}
	invitee, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 702, FirstName: "Invitee", Username: "invitee"}, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveAffiliateConfig(ctx, inviter.ID, affiliates.ConfigInput{ExpectedVersion: 1, Tiers: []affiliates.Tier{
		{Name: "Starter", Threshold: 0, Enabled: true, CommissionEnabled: true, CommissionBPS: 1000, Reward: affiliates.Reward{Kind: "none"}},
		{Name: "Partner", Threshold: 1, Enabled: true, CommissionEnabled: true, CommissionBPS: 2000, Reward: affiliates.Reward{Kind: "txb", TXBMinor: 500}},
	}}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	first := createTestPaymentOrder(t, store, invitee.ID, "ezpay", 1001, time.Now().UTC())
	if _, changed, err := store.SettlePayment(ctx, "ezpay", "affiliate-event-1", "hash-1", first.ID, "trade-aff-1", "", time.Now().UTC()); err != nil || !changed {
		t.Fatalf("SettlePayment(first) = %v, %v", changed, err)
	}
	var balance, commission int64
	if err := store.DB().QueryRow(`SELECT txb_minor FROM balances WHERE user_id=?`, inviter.ID).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRow(`SELECT commission_txb_minor FROM affiliate_settlements WHERE invited_user_id=?`, invitee.ID).Scan(&commission); err != nil {
		t.Fatal(err)
	}
	if commission != 100 || balance != 600 {
		t.Fatalf("commission/balance = %d/%d, want 100/600", commission, balance)
	}
	second := createTestPaymentOrder(t, store, invitee.ID, "ezpay", 2000, time.Now().UTC())
	if _, _, err := store.SettlePayment(ctx, "ezpay", "affiliate-event-2", "hash-2", second.ID, "trade-aff-2", "", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var settlements, awards int
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM affiliate_settlements`).Scan(&settlements); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM affiliate_tier_awards`).Scan(&awards); err != nil {
		t.Fatal(err)
	}
	if settlements != 1 || awards != 1 {
		t.Fatalf("settlements/awards = %d/%d, want 1/1", settlements, awards)
	}
}

func TestCompactAndPrunePreservesAffiliatePaymentEvidence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	createTestUser(t, store, 703)
	if _, accepted, err := store.AcceptAffiliateReferral(ctx, 704, 703, time.Now().UTC()); err != nil || !accepted {
		t.Fatalf("AcceptAffiliateReferral() = %v, %v", accepted, err)
	}
	invitee, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 704, FirstName: "Invitee", Username: "invitee"}, false)
	if err != nil {
		t.Fatal(err)
	}
	settledAt := time.Date(2026, time.August, 1, 2, 0, 0, 0, time.UTC)
	order := createTestPaymentOrder(t, store, invitee.ID, "ezpay", 1_000, settledAt)
	if _, changed, err := store.SettlePayment(ctx, "ezpay", "affiliate-retention", "hash", order.ID, "trade-retention", "", settledAt); err != nil || !changed {
		t.Fatalf("SettlePayment() = %v, %v", changed, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET updated_at=? WHERE id=?`, stamp(settledAt), order.ID); err != nil {
		t.Fatalf("age payment order: %v", err)
	}

	now := settledAt.Add(8 * 24 * time.Hour)
	counts, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("CompactAndPrune(): %v", err)
	}
	if counts["payment_orders"] != 0 {
		t.Fatalf("pruned payment orders = %d, want 0", counts["payment_orders"])
	}
	if _, err := store.PaymentOrderByID(ctx, order.ID); err != nil {
		t.Fatalf("PaymentOrderByID() = %v, want preserved affiliate evidence", err)
	}
	var settlements int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM affiliate_settlements WHERE payment_order_id=?`, order.ID).Scan(&settlements); err != nil {
		t.Fatalf("count affiliate settlements: %v", err)
	}
	if settlements != 1 {
		t.Fatalf("affiliate settlements = %d, want 1", settlements)
	}
}
