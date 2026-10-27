package database

import (
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
	"time"
)

func fixedPersonalPricing(in personalPriceInput) bool {
	return !in.Policy.DynamicEnabled || in.Policy.PurchaseMode == "FIXED" && !in.Policy.PersonalDiscounts
}
func fixedPersonalSnapshot(in *personalPriceInput, combo string, squads []string, now time.Time) error {
	raw, err := json.Marshal([]any{pricing.StrategyVersion, in.User, in.Kind, in.Product, combo, uniqueSorted(squads), in.Policy, in.Public, in.Coupons, now.UTC().Truncate(6 * time.Hour)})
	if err != nil {
		return err
	}
	in.Context = compute.Digest(raw)
	in.Version = in.Context
	in.Support = &pricing.PriceSupport{}
	return nil
}
func fixedPersonalDecision(in personalPriceInput, now time.Time) model.PricingDecision {
	discount, _ := pricingCouponDiscount(in.Coupons, in.Public)
	return model.PricingDecision{DynamicEnabled: in.Policy.DynamicEnabled, CouponFingerprint: couponFingerprint(in.Coupons), Version: in.Version, Status: "UNCHANGED", Reason: "FIXED_PRICE", PublicPrice: model.TXBMoney(in.Public), PersonalPrice: model.TXBMoney(in.Public), NetPrice: model.TXBMoney(in.Public - discount), ModelVersion: "fixed-v1", Calibration: "NOT_APPLICABLE", GeneratedAt: now, ValidUntil: in.Until}
}
