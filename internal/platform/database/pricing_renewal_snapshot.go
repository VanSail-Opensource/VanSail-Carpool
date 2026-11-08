package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing/renewal"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

type renewalPriceSnapshot struct {
	Contract      renewal.Contract
	Policy        pricing.Policy
	Public        int64
	Due           time.Time
	Coupon        *coupons.Coupon
	Health, Hash  string
	PublicEnabled bool
}

func (s *Store) renewalPriceSnapshotTx(ctx context.Context, tx *sql.Tx, c renewal.Contract, now time.Time) (renewalPriceSnapshot, error) {
	in := renewalPriceSnapshot{Contract: c, Public: c.Price, Health: "TELEMETRY_STALE"}
	var until string
	var recurring int
	var grant sql.NullString
	var reward sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT valid_until,recurring_discount_attached,coupon_grant_id,reward_renewal_price_minor FROM purchases WHERE id=$1`, c.PurchaseID).Scan(&until, &recurring, &grant, &reward)
	if err != nil {
		return in, err
	}
	in.Due, err = parseStamp(until)
	if err != nil {
		return in, err
	}
	in.Policy, err = loadPricingPolicy(ctx, tx, "combo", c.ProductID, c.Price)
	if err != nil {
		return in, err
	}
	if c.Mode == "DYNAMIC" && in.Policy.DynamicEnabled && in.Policy.RenewalMode == "DYNAMIC" {
		if price, e := approvedPricingTx(ctx, tx, in.Policy, in.Policy.FixedPrice, now); e == nil {
			var addons int64
			for _, a := range c.Addons {
				addons += a.Price
			}
			in.Public = price + addons
		}
	}
	nodes, err := pricingNodesTx(ctx, tx, "combo", c.ProductID)
	if err != nil {
		return in, err
	}
	for _, addon := range c.Addons {
		extra, e := pricingNodesTx(ctx, tx, "addon", addon.ID)
		if e != nil {
			return in, e
		}
		nodes = append(nodes, extra...)
	}
	nodes = uniqueSorted(nodes)
	if s.pricingHealth.Safe(nodes, now) {
		in.Health = "HEALTHY"
	}
	if recurring == 1 && grant.Valid {
		g, e := grantByIDTx(ctx, tx, grant.String)
		if e != nil {
			return in, e
		}
		in.Coupon = &coupons.Coupon{CouponInput: coupons.CouponInput{ID: g.Coupon.ID, Kind: coupons.KindPurchaseRecurring, DiscountMode: g.Coupon.DiscountMode, ValueMinorOrBPS: g.Coupon.ValueMinorOrBPS, PercentCapMinor: g.Coupon.PercentCapMinor}}
	}
	frozen, err := pricingPublicationFrozenTx(ctx, tx, in.Policy, now)
	if err != nil {
		return in, err
	}
	in.PublicEnabled = c.Mode == "DYNAMIC" && in.Policy.DynamicEnabled && in.Policy.RenewalMode == "DYNAMIC" && in.Health == "HEALTHY" && !reward.Valid && !frozen
	raw, err := json.Marshal(in)
	if err != nil {
		return in, err
	}
	in.Hash = compute.Digest(raw)
	return in, nil
}

func renewalNet(in renewalPriceSnapshot, price int64) (int64, error) {
	if in.Coupon == nil {
		return price, nil
	}
	discount, err := attachedDiscountMinor(*in.Coupon, price)
	return price - discount, err
}
