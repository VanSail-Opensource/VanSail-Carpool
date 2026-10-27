package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
)

func TestCheckoutRevalidatesCouponBeforeConsumption(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, s, 49552)
	now := time.Now().UTC().Truncate(5 * time.Minute).Add(time.Minute)
	combo := saveTestCombo(t, s, "coupon-review", 1000, 30)
	if _, err := s.AdjustBalance(ctx, user.ID, 5000, "coupon-review-funds", "seed", now); err != nil {
		t.Fatal(err)
	}
	coupon, err := s.SaveCoupon(ctx, coupons.CouponInput{Code: "REVIEWCAP", Name: "Reviewed coupon", Kind: coupons.KindPurchaseOnce, DiscountMode: coupons.DiscountPercent, ValueMinorOrBPS: 5000, Active: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := s.RedeemCoupon(ctx, user.ID, coupon.Code, "grant", now)
	if err != nil || grant.Grant == nil {
		t.Fatal(grant, err)
	}
	input := PurchaseInput{UserID: user.ID, ComboID: combo.ID, CouponGrantID: grant.Grant.ID, IdempotencyKey: "coupon-review"}
	quote, err := s.QuotePurchase(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	input.QuoteVersion = quote.QuoteVersion
	coupon.ValueMinorOrBPS = 4000
	if _, err = s.SaveCoupon(ctx, coupon.CouponInput, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreatePurchase(ctx, input, now.Add(time.Second)); !errors.Is(err, ErrPricingQuoteChanged) {
		t.Fatal(err)
	}
	var uses int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM coupon_uses WHERE grant_id=?`, grant.Grant.ID).Scan(&uses); err != nil || uses != 0 {
		t.Fatal(uses, err)
	}
	balance, err := s.Balance(ctx, user.ID)
	if err != nil || balance.MinorInt64() != 5000 {
		t.Fatal(balance, err)
	}
}

func TestQueuedCheckoutRejectsChangedCampaignWithoutChangingPaidPredecessor(t *testing.T) {
	s, user, actor, source, policy, now := pricingRenewalFixture(t)
	ctx := context.Background()
	input := PurchaseInput{UserID: user.ID, ComboID: policy.ProductID, IdempotencyKey: "campaign-review"}
	quote, err := s.QuotePurchase(ctx, input, now)
	if err != nil || !quote.Queued {
		t.Fatal(quote, err)
	}
	input.QuoteVersion = quote.QuoteVersion
	_, err = s.CreatePricingIntervention(ctx, actor.ID, PricingIntervention{ScopeType: "user_product", ScopeID: user.ID + ":combo:" + policy.ProductID, Command: "PERSONAL_DISCOUNT", DiscountBPS: 1000, Budget: 1000, Reason: "campaign changed after review", ExpiresAt: now.Add(time.Hour)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreatePurchase(ctx, input, now); !errors.Is(err, ErrPricingQuoteChanged) {
		t.Fatal(err)
	}
	paid, err := s.PurchaseByID(ctx, source.ID)
	if err != nil || paid.PriceTXBMinor != source.PriceTXBMinor || !paid.ValidUntil.Equal(source.ValidUntil) {
		t.Fatal(paid, err)
	}
	var spend int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_discount_spend WHERE user_id=?`, user.ID).Scan(&spend); err != nil || spend != 0 {
		t.Fatal(spend, err)
	}
}
