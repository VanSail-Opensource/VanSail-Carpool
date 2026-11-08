package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

type checkoutPriceInput struct {
	User, Kind, Product, Version, Unavailable string
	Public, BasketOther                       int64
	GlobalPending                             bool
	Policy                                    pricing.Policy
	Coupons                                   []coupons.Grant
	Until                                     time.Time
}

func checkoutPriceSnapshotTx(ctx context.Context, tx *sql.Tx, user, kind, product, combo string, squads []string, now time.Time) (checkoutPriceInput, error) {
	in := checkoutPriceInput{User: user, Kind: kind, Product: product, Coupons: []coupons.Grant{}, Until: now.Add(6 * time.Hour)}
	fixed, err := pricingReference(ctx, tx, kind, product)
	if err != nil {
		return in, err
	}
	in.Policy, err = loadPricingPolicy(ctx, tx, kind, product, fixed)
	if err != nil {
		return in, err
	}
	in.Public = fixed
	if in.Policy.DynamicEnabled && in.Policy.PurchaseMode == "DYNAMIC" {
		price, err := approvedPricingTx(ctx, tx, in.Policy, fixed, now)
		if err == nil {
			in.Public = price
		} else {
			var code pricing.Code
			if !errors.As(err, &code) {
				return in, err
			}
			in.Unavailable = string(code)
		}
		cutoff := now.UTC().Truncate(24 * time.Hour)
		if kind == "payg" {
			cutoff = now.UTC().Truncate(time.Minute)
		}
		var state string
		err = tx.QueryRowContext(ctx, `SELECT state FROM pricing_decisions WHERE product_type=$1 AND product_id=$2 AND policy_version>=$3 AND generated_at>=$4 ORDER BY generated_at DESC LIMIT 1`, kind, product, in.Policy.Version, pricingStamp(cutoff)).Scan(&state)
		if errors.Is(err, sql.ErrNoRows) {
			in.GlobalPending = true
		} else if err != nil {
			return in, err
		}
	}
	bucket := now.UTC().Truncate(6 * time.Hour)
	if kind == "payg" {
		bucket = now.UTC().Truncate(time.Minute)
		in.Until = bucket.Add(time.Minute)
	} else {
		in.Coupons, err = pricingCouponsTx(ctx, tx, user, combo, squads, now)
		if err != nil {
			return in, err
		}
		if err = pricingBasketTx(ctx, tx, &in, combo, squads, now); err != nil {
			return in, err
		}
	}
	for _, grant := range in.Coupons {
		if grant.Coupon.ExpiresAt != nil && grant.Coupon.ExpiresAt.Before(in.Until) {
			in.Until = *grant.Coupon.ExpiresAt
		}
	}
	raw, err := json.Marshal([]any{"public-checkout-v1", user, kind, product, combo, uniqueSorted(squads), in.Policy, in.Public, in.Coupons, in.BasketOther, bucket})
	if err != nil {
		return in, err
	}
	in.Version = compute.Digest(raw)
	return in, nil
}
