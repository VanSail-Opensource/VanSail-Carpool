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

type personalPriceInput struct {
	User, Kind, Product, Context, Version, ModelID, Calibration string
	Public                                                      int64
	Unavailable                                                 string
	CouponSamples                                               int
	GlobalPending                                               bool
	GlobalValidated                                             bool
	Policy                                                      pricing.Policy
	Model                                                       pricing.ResponseModel
	Observation                                                 pricing.ResponseObservation
	Coupons                                                     []coupons.Grant
	Until                                                       time.Time
	Support                                                     *pricing.PriceSupport
	BasketOther                                                 int64
	BasketCost                                                  float64
	CostsComplete                                               bool
}

func personalPriceSnapshotTx(ctx context.Context, tx *sql.Tx, user, kind, product, combo string, squads []string, now time.Time) (personalPriceInput, error) {
	in := personalPriceInput{User: user, Kind: kind, Product: product, Coupons: []coupons.Grant{}, Calibration: "PRIOR_ONLY", ModelID: "purchase-prior-v1", Until: now.Add(6 * time.Hour)}
	fixed, err := pricingReference(ctx, tx, kind, product)
	if err != nil {
		return in, err
	}
	in.Policy, err = loadPricingPolicy(ctx, tx, kind, product, fixed)
	if err != nil {
		return in, err
	}
	in.Public = fixed
	if fixedPersonalPricing(in) {
		if batch, ok := ctx.Value(pricingReadBatchKey{}).(*pricingReadBatch); ok && batch.User == user && kind != "payg" {
			in.Coupons, err = pricingCouponsTx(ctx, tx, user, combo, squads, now)
			if err != nil {
				return in, err
			}
		}
		err = fixedPersonalSnapshot(&in, combo, squads, now)
		return in, err
	}
	if in.Policy.DynamicEnabled && in.Policy.PurchaseMode == "DYNAMIC" {
		price, e := approvedPricingTx(ctx, tx, in.Policy, fixed, now)
		if e == nil {
			in.Public = price
		} else {
			var code pricing.Code
			if !errors.As(e, &code) {
				return in, e
			}
			in.Unavailable = string(code)
		}
	}
	if in.Policy.DynamicEnabled && in.Policy.PurchaseMode == "DYNAMIC" {
		cutoff := now.UTC().Truncate(24 * time.Hour)
		if kind == "payg" {
			cutoff = now.UTC().Truncate(time.Minute)
		}
		var state string
		e := tx.QueryRowContext(ctx, `SELECT state FROM pricing_decisions WHERE product_type=? AND product_id=? AND policy_version>=? AND generated_at>=? ORDER BY generated_at DESC LIMIT 1`, kind, product, in.Policy.Version, pricingStamp(cutoff)).Scan(&state)
		if errors.Is(e, sql.ErrNoRows) {
			in.GlobalPending = true
		} else if e != nil {
			return in, e
		}
		in.GlobalValidated = state == "PUBLISHED"
	}
	if kind == "payg" {
		in.Until = now.UTC().Truncate(time.Minute).Add(time.Minute)
	} else {
		in.Coupons, err = pricingCouponsTx(ctx, tx, user, combo, squads, now)
		if err != nil {
			return in, err
		}
	}
	for _, g := range in.Coupons {
		if g.Coupon.ExpiresAt != nil && g.Coupon.ExpiresAt.Before(in.Until) {
			in.Until = *g.Coupon.ExpiresAt
		}
	}
	var watermark, raw, diagnostics string
	in.Observation, _, watermark, err = pricingExposureInputs(ctx, tx, user, now)
	if err != nil {
		return in, err
	}
	err = tx.QueryRowContext(ctx, `SELECT id,parameters_json,calibration_status,diagnostics_json FROM pricing_model_versions WHERE kind=? AND id LIKE ? AND frozen=0 AND created_at<=? ORDER BY created_at DESC LIMIT 1`, "response:"+kind+":"+product, "%:"+pricing.StrategyVersion+":%", pricingStamp(now)).Scan(&in.ModelID, &raw, &in.Calibration, &diagnostics)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return in, err
	}
	if err == nil {
		var evidence struct {
			Coupons int `json:"couponSamples"`
		}
		if err = json.Unmarshal([]byte(diagnostics), &evidence); err != nil {
			return in, err
		}
		in.CouponSamples = evidence.Coupons
	}
	if err == nil {
		if err = json.Unmarshal([]byte(raw), &in.Model); err != nil {
			return in, err
		}
	}
	in.Support, err = pricingModelSupportTx(ctx, tx, in.ModelID)
	if err != nil {
		return in, err
	}
	if err = pricingBasketTx(ctx, tx, &in, combo, squads, now); err != nil {
		return in, err
	}
	scope, _ := json.Marshal([]any{combo, uniqueSorted(squads), in.BasketOther})
	in.Context = compute.Digest(scope)
	// Only immutable decision evidence is stored. Coupon definitions, limits,
	// expiry, grant use counts and source watermarks all invalidate stale offers.
	evidence, err := json.Marshal([]any{pricing.StrategyVersion, compute.Version, user, kind, product, in.Context, in.Policy, in.Public, in.ModelID, in.Coupons, in.BasketCost, in.CostsComplete, watermark, now.UTC().Truncate(func() time.Duration {
		if kind == "payg" {
			return time.Minute
		}
		return 6 * time.Hour
	}()).Format(time.RFC3339)})
	if err != nil {
		return in, err
	}
	in.Version = compute.Digest(evidence)
	return in, nil
}
