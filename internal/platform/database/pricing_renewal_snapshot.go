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
	Contract            renewal.Contract
	Policy              pricing.Policy
	Public              int64
	Due                 time.Time
	Response            pricing.ResponseModel
	Observation         pricing.ResponseObservation
	Coupon              *coupons.Coupon
	Model, Health, Hash string
	Cost, Relationships float64
	Validated           bool
	Support             *pricing.PriceSupport
}

func (s *Store) renewalPriceSnapshotTx(ctx context.Context, tx *sql.Tx, c renewal.Contract, now time.Time) (renewalPriceSnapshot, error) {
	in := renewalPriceSnapshot{Contract: c, Public: c.Price, Health: "TELEMETRY_STALE"}
	var until string
	var recurring int
	var grant sql.NullString
	var reward sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT valid_until,recurring_discount_attached,coupon_grant_id,reward_renewal_price_minor FROM purchases WHERE id=?`, c.PurchaseID).Scan(&until, &recurring, &grant, &reward)
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
	in.Response, in.Model, err = pricingRenewalModelTx(ctx, tx, c.ProductID, now)
	if err != nil {
		return in, err
	}
	in.Support, err = pricingModelSupportTx(ctx, tx, in.Model)
	if err != nil {
		return in, err
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
	var complete bool
	in.Cost, in.Relationships, complete, err = pricingRenewalEconomicsTx(ctx, tx, c.UserID, c.ProductID, c.PurchaseID, nodes, now)
	if err != nil {
		return in, err
	}
	if recurring == 1 && grant.Valid {
		g, e := grantByIDTx(ctx, tx, grant.String)
		if e != nil {
			return in, e
		}
		in.Coupon = &coupons.Coupon{CouponInput: coupons.CouponInput{ID: g.Coupon.ID, Kind: coupons.KindPurchaseRecurring, DiscountMode: g.Coupon.DiscountMode, ValueMinorOrBPS: g.Coupon.ValueMinorOrBPS, PercentCapMinor: g.Coupon.PercentCapMinor}}
	}
	in.Observation, _, _, err = pricingExposureInputs(ctx, tx, c.UserID, now)
	if err != nil {
		return in, err
	}
	in.Validated = c.Mode == "DYNAMIC" && in.Response.Validated && in.Health == "HEALTHY" && complete && !reward.Valid
	if in.Coupon != nil && in.Validated {
		var raw string
		if err = tx.QueryRowContext(ctx, `SELECT diagnostics_json FROM pricing_model_versions WHERE id=?`, in.Model).Scan(&raw); err != nil {
			return in, err
		}
		var evidence struct {
			Coupons int `json:"couponSamples"`
		}
		if err = json.Unmarshal([]byte(raw), &evidence); err != nil {
			return in, err
		}
		in.Validated = evidence.Coupons >= 30
	}
	frozen, err := pricingPublicationFrozenTx(ctx, tx, in.Policy, now)
	if err != nil {
		return in, err
	}
	in.Validated = in.Validated && !frozen
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
