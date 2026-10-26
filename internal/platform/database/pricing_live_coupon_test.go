package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestLiveSquadCouponDebitsOnceAndKeepsRenewalAttachment(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, s, 49773)
	now := time.Now().UTC()
	addon := saveTestSquad(t, s, "coupon-live", 300, true)
	combo := saveTestCombo(t, s, "live-source", 1000, 30)
	if _, err := s.AdjustBalance(ctx, user.ID, 5000, "live-coupon-funds", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := s.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.SaveCoupon(ctx, coupons.CouponInput{Code: "LIVEHALF", Name: "Live", Kind: coupons.KindPurchaseOnce, DiscountMode: coupons.DiscountPercent, ValueMinorOrBPS: 5000, EligibleSquadIDs: []string{addon.ID}, Active: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := s.RedeemCoupon(ctx, user.ID, c.Code, "grant", now)
	if err != nil || grant.Grant == nil {
		t.Fatal(grant, err)
	}
	input := PurchaseAddonInput{UserID: user.ID, PurchaseID: source.ID, AddonSquadIDs: []string{addon.ID}, CouponGrantID: grant.Grant.ID, IdempotencyKey: "addition"}
	quote, err := s.QuotePurchaseAddons(ctx, input, now)
	if err != nil || quote.GrossPriceTXBMinor != 300 || quote.PriceTXBMinor != 150 {
		t.Fatal(quote, err)
	}
	added, err := s.AddPurchaseAddons(ctx, input, now)
	if err != nil || added.PriceTXBMinor != 1150 || added.GrossPriceTXBMinor != 1300 || added.CouponGrantID != nil {
		t.Fatal(added, err)
	}
	replay, err := s.AddPurchaseAddons(ctx, input, now)
	if err != nil || replay.PriceTXBMinor != 1150 {
		t.Fatal(replay, err)
	}
	var uses, charges int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM coupon_uses WHERE grant_id=?`, grant.Grant.ID).Scan(&uses); err != nil || uses != 1 {
		t.Fatal(uses, err)
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind='purchase_addon_debit'`, user.ID).Scan(&charges); err != nil || charges != 1 {
		t.Fatal(charges, err)
	}
}
