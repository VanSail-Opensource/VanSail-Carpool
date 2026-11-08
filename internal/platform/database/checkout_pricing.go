package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

var ErrPricingDecisionPending = errors.New("purchase pricing decision is pending or stale")

// PrepareCheckoutPricing performs only public product calculation before the
// wallet lock. Coupon eligibility is owner scoped and re-read at settlement.
func (s *Store) PrepareCheckoutPricing(ctx context.Context, user, kind, product, combo string, squads []string, now time.Time) (model.PricingDecision, error) {
	if _, ok := ctx.Value(pricingReadBatchKey{}).(*pricingReadBatch); !ok {
		var err error
		ctx, err = s.pricingReadContext(ctx, user, now)
		if err != nil {
			return model.PricingDecision{}, err
		}
	}
	full, _ := ctx.Value(pricingBasketKey{}).(bool)
	key, err := json.Marshal([]any{user, kind, product, combo, uniqueSorted(squads), full, now.UTC().Truncate(time.Minute)})
	if err != nil {
		return model.PricingDecision{}, err
	}
	return pricingCached(ctx, s, "checkout:"+string(key), 0, func() (model.PricingDecision, error) {
		for attempt := 0; attempt < 2; attempt++ {
			tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			if err != nil {
				return model.PricingDecision{}, err
			}
			in, err := checkoutPriceSnapshotTx(ctx, tx, user, kind, product, combo, squads, now)
			tx.Rollback()
			if err != nil {
				return model.PricingDecision{}, err
			}
			decision, err := checkoutDecision(in, now)
			if err != nil {
				return decision, err
			}
			if !in.GlobalPending {
				return decision, nil
			}
			if attempt == 0 {
				err = s.PricingOptimizeProduct(ctx, kind, product, now)
				if err != nil && !errors.Is(err, ErrPricingComputePending) {
					return decision, err
				}
				if err == nil {
					ctx = pricingFreshContext(ctx)
					continue
				}
			}
			return decision, nil
		}
		return model.PricingDecision{}, ErrPricingDecisionPending
	})
}

func checkoutDecision(in checkoutPriceInput, now time.Time) (model.PricingDecision, error) {
	gross := in.Public + in.BasketOther
	discount, err := pricingCouponDiscount(in.Coupons, gross)
	if err != nil {
		return model.PricingDecision{}, err
	}
	d := model.PricingDecision{DynamicEnabled: in.Policy.DynamicEnabled, CouponFingerprint: couponFingerprint(in.Coupons), Version: in.Version, Status: "UNCHANGED", Reason: "FIXED_PRICE", PublicPrice: model.TXBMoney(in.Public), PersonalPrice: model.TXBMoney(in.Public), NetPrice: model.TXBMoney(gross - discount), ModelVersion: "public-checkout-v1", Calibration: "NOT_APPLICABLE", GeneratedAt: now, ValidUntil: in.Until}
	if in.Policy.DynamicEnabled && in.Policy.PurchaseMode == "DYNAMIC" {
		d.Reason = "PUBLIC_PRICE_ACCEPTED"
	}
	if in.Unavailable != "" {
		d.Status, d.Reason = "STALE", in.Unavailable
	}
	if in.GlobalPending {
		d.Status, d.Reason = "PENDING", "CALCULATION_PENDING"
	}
	if in.BasketOther > 0 {
		total := model.TXBMoney(gross)
		d.BasketGrossPrice = &total
	}
	return d, nil
}

func checkoutProductPriceTx(ctx context.Context, tx *sql.Tx, user, kind, product, combo string, squads []string, now time.Time) (int64, *model.PricingDecision, error) {
	in, err := checkoutPriceSnapshotTx(ctx, tx, user, kind, product, combo, squads, now)
	if err != nil {
		return 0, nil, err
	}
	if in.Unavailable != "" || in.GlobalPending {
		return 0, nil, ErrPricingDecisionPending
	}
	d, err := checkoutDecision(in, now)
	return in.Public, &d, err
}

func moneyMinor(m model.Money) int64 { value, _ := strconv.ParseInt(m.Minor, 10, 64); return value }
