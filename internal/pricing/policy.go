// Package pricing owns optional pricing recommendations and policy constraints.
// It never debits wallets or creates entitlements.
package pricing

import (
	"errors"
	"math"
	"time"
)

const Scale int64 = 10000
const MaxPrice int64 = 1_000_000_000_000
const MaxTotalPrice int64 = 101 * MaxPrice

// Policy is an explicit per-product opt-in. Absence means authoritative fixed pricing.
type Policy struct {
	ProductType         string    `json:"productType"`
	ProductID           string    `json:"productId"`
	DynamicEnabled      bool      `json:"dynamicEnabled"`
	PurchaseMode        string    `json:"purchasePricingMode"`
	RenewalMode         string    `json:"renewalPricingMode"`
	PersonalDiscounts   bool      `json:"personalDiscountsEnabled"`
	FixedPrice          int64     `json:"fixedPriceMinor,string"`
	MaxIncreaseBPS      int64     `json:"maxIncreaseBps"`
	MaxDiscountBPS      int64     `json:"maxDiscountBps"`
	OverrideStatus      string    `json:"overrideStatus"`
	LastApprovedVersion string    `json:"lastApprovedPriceVersion"`
	Version             int64     `json:"version"`
	UpdatedBy           string    `json:"updatedBy"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// DefaultPolicy keeps every product fixed until an administrator saves an opt-in.
func DefaultPolicy(kind, id string, fixed int64) Policy {
	return Policy{ProductType: kind, ProductID: id, FixedPrice: fixed, PurchaseMode: "FIXED", RenewalMode: "FIXED", OverrideStatus: "NONE"}
}

// Validate checks policy shape and overflow-safe monetary bounds.
func (p Policy) Validate() error {
	if (p.ProductType != "combo" && p.ProductType != "payg" && p.ProductType != "addon") || p.ProductID == "" || len(p.ProductID) > 128 || p.Version < 0 || p.FixedPrice < 0 || p.FixedPrice > MaxPrice || p.MaxIncreaseBPS < 0 || p.MaxIncreaseBPS > Scale || p.MaxDiscountBPS < 0 || p.MaxDiscountBPS > Scale {
		return errors.New("invalid pricing policy")
	}
	if (p.PurchaseMode != "FIXED" && p.PurchaseMode != "DYNAMIC") || (p.RenewalMode != "FIXED" && p.RenewalMode != "DYNAMIC") || (p.OverrideStatus != "NONE" && p.OverrideStatus != "FROZEN" && p.OverrideStatus != "MANUAL") {
		return errors.New("invalid pricing mode")
	}
	if !p.DynamicEnabled && (p.PurchaseMode != "FIXED" || p.RenewalMode != "FIXED") {
		return errors.New("dynamic pricing requires explicit activation")
	}
	if p.ProductType != "combo" && p.RenewalMode != "FIXED" {
		return errors.New("only Combo products have independent renewal contracts")
	}
	return nil
}

// BoundPrice enforces reference-relative policy constraints using integer arithmetic.
func BoundPrice(reference, proposed, increase, discount int64) (int64, error) {
	if reference < 0 || reference > MaxPrice || proposed < 0 || proposed > MaxPrice || increase < 0 || increase > Scale || discount < 0 || discount > Scale {
		return 0, errors.New("invalid price bounds")
	}
	low := reference * (Scale - discount) / Scale
	high := reference + reference*increase/Scale
	return min(max(proposed, low), high), nil
}

// Probability clamps finite statistical results to a probability.
func Probability(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return min(1, max(0, value))
}
