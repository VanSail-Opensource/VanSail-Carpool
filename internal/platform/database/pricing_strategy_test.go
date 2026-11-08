package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"testing"
	"time"
)

func TestPricingNativeAllocationConservesSharedMachineCost(t *testing.T) {
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	group := saveTestSquad(t, s, "shared-cost", 100, true)
	if err := s.SaveXNodeGroup(ctx, xnode.Group{ID: group.GroupID, Name: "Shared", Enabled: true, Bindings: []xnode.GroupBinding{{NodeID: "node", InboundTag: "in"}}}); err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, s, "cost-source", 1000, 30, group.ID)
	if _, err := s.AdjustBalance(ctx, identity.UserID, 5000, "cost-funds", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := s.CreatePurchase(ctx, PurchaseInput{UserID: identity.UserID, ComboID: combo.ID, IdempotencyKey: "cost-purchase"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE xnode_periods SET purchase_id=$1 WHERE id=$2`, purchase.ID, period.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO purchase_addons(purchase_id,group_id,charged_txb_minor) VALUES($1,$2,100)`, purchase.ID, group.GroupID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO xnode_daily_usage(proxy_user_id,node_id,period_id,date,upload_bytes,download_bytes) VALUES($1,'node',$2,$3,$4,0)`, identity.ProxyUserID, period.ID, now.AddDate(0, 0, -1).Format(time.DateOnly), xnode.PAYGGB); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	a, err := pricingAllocationsTx(ctx, tx, "combo", combo.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := pricingAllocationsTx(ctx, tx, "addon", group.GroupID, now)
	if err != nil {
		t.Fatal(err)
	}
	if a["node"].Product+b["node"].Product != float64(xnode.PAYGGB) || a["node"].Allocated != a["node"].Total {
		t.Fatal(a, b)
	}
}
func TestCheckoutCouponsUseWholeBasketOnce(t *testing.T) {
	cap := int64(300)
	in := checkoutPriceInput{Public: 1000, BasketOther: 1000, Policy: pricing.DefaultPolicy("combo", "c", 1000), Coupons: []coupons.Grant{{Coupon: coupons.Coupon{CouponInput: coupons.CouponInput{Kind: coupons.KindPurchaseOnce, DiscountMode: coupons.DiscountPercent, ValueMinorOrBPS: 5000, PercentCapMinor: &cap}}}}}
	decision, err := checkoutDecision(in, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if decision.BasketGrossPrice == nil || decision.BasketGrossPrice.MinorInt64() != 2000 || decision.NetPrice.MinorInt64() != 1700 || decision.PersonalPrice != decision.PublicPrice {
		t.Fatal(decision)
	}
}
