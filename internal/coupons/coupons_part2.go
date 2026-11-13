package coupons

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
)

type PurchaseContext struct {
	UserID          string
	GrantID         string
	ComboID         string
	AddonSquadIDs   []string
	GrossPriceMinor int64
}

// Discount is a server-calculated coupon quote or consumed purchase effect.

type Discount struct {
	GrantID       string `json:"grantId"`
	CouponID      string `json:"couponId"`
	CouponCode    string `json:"couponCode"`
	GrossMinor    int64  `json:"grossMinor"`
	DiscountMinor int64  `json:"discountMinor"`
	NetMinor      int64  `json:"netMinor"`
	Recurring     bool   `json:"recurring"`
}

// EligibleFor returns whether a coupon applies to a combo or any selected add-on.

func (coupon Coupon) EligibleFor(comboID string, addonSquadIDs []string) bool {
	if len(coupon.EligibleComboIDs) == 0 && len(coupon.EligibleSquadIDs) == 0 {
		return true
	}
	for _, allowed := range coupon.EligibleComboIDs {
		if allowed == comboID {
			return true
		}
	}
	selected := make(map[string]struct{}, len(addonSquadIDs))
	for _, id := range addonSquadIDs {
		selected[id] = struct{}{}
	}
	for _, allowed := range coupon.EligibleSquadIDs {
		if _, ok := selected[allowed]; ok {
			return true
		}
	}
	return false
}

// CalculateDiscount applies fixed or basis-point pricing without floating point.

func CalculateDiscount(coupon Coupon, grossMinor int64) (int64, error) {
	if grossMinor < 0 {
		return 0, fmt.Errorf("%w: gross price cannot be negative", ErrInvalidInput)
	}
	if coupon.Kind != KindPurchaseRecurring && coupon.Kind != KindPurchaseOnce {
		return 0, fmt.Errorf("%w: coupon is not a purchase discount", ErrInvalidInput)
	}
	// Stored definitions can predate current administrator validation. Never turn
	// an invalid discount into a surcharge or silently widen its allowed value.
	if coupon.ValueMinorOrBPS <= 0 {
		return 0, fmt.Errorf("%w: purchase discount must be positive", ErrInvalidInput)
	}
	var discount int64
	switch coupon.DiscountMode {
	case DiscountFixed:
		if coupon.PercentCapMinor != nil {
			return 0, fmt.Errorf("%w: fixed discount cannot have a percentage cap", ErrInvalidInput)
		}
		discount = coupon.ValueMinorOrBPS
	case DiscountPercent:
		if coupon.ValueMinorOrBPS > 10_000 || coupon.PercentCapMinor != nil && *coupon.PercentCapMinor <= 0 {
			return 0, fmt.Errorf("%w: invalid percentage discount or cap", ErrInvalidInput)
		}
		value, err := multiplyDivideFloor(grossMinor, coupon.ValueMinorOrBPS, 10_000)
		if err != nil {
			return 0, err
		}
		discount = value
		if coupon.PercentCapMinor != nil && discount > *coupon.PercentCapMinor {
			discount = *coupon.PercentCapMinor
		}
	default:
		return 0, fmt.Errorf("%w: unsupported discount mode", ErrInvalidInput)
	}
	if discount > grossMinor {
		discount = grossMinor
	}
	return discount, nil
}

// CalculateBalanceMultiplyCredit returns floor(balance*factor)-balance.

func CalculateBalanceMultiplyCredit(balanceMinor, multiplierBPS int64) (int64, error) {
	if balanceMinor < 0 || multiplierBPS <= 10_000 {
		return 0, fmt.Errorf("%w: invalid balance multiplier", ErrInvalidInput)
	}
	finalBalance, err := multiplyDivideFloor(balanceMinor, multiplierBPS, 10_000)
	if err != nil {
		return 0, err
	}
	return finalBalance - balanceMinor, nil
}

func multiplyDivideFloor(left, right, divisor int64) (int64, error) {
	if left < 0 || right < 0 || divisor <= 0 {
		return 0, fmt.Errorf("%w: invalid fixed-point operands", ErrInvalidInput)
	}
	value := new(big.Int).Mul(big.NewInt(left), big.NewInt(right))
	value.Quo(value, big.NewInt(divisor))
	if !value.IsInt64() {
		return 0, fmt.Errorf("%w: fixed-point result overflows int64", ErrInvalidInput)
	}
	return value.Int64(), nil
}

func hasBlank(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func uniqueSorted(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[strings.TrimSpace(value)] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
