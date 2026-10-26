package activity

import (
	"fmt"
	"strings"
)

// RewardKind identifies the effect of a lucky-draw prize.
type RewardKind string

const (
	// RewardNone records a draw without an additional reward.
	RewardNone RewardKind = "none"
	// RewardTXBDelta changes the member's TXB balance by a signed amount.
	RewardTXBDelta RewardKind = "txb_delta"
	// RewardCouponGrant adds a coupon grant to the member's wallet.
	RewardCouponGrant RewardKind = "coupon_grant"
	// RewardSubscriptionExtension extends the current or next subscription.
	RewardSubscriptionExtension RewardKind = "subscription_extension"
	RewardEntitlementGrant      RewardKind = "entitlement_grant"
	RewardSquadAccess           RewardKind = "squad_access"
	RewardCoreComboSwitch       RewardKind = "core_combo_switch"
	RewardTrafficGrant          RewardKind = "traffic_grant"
	RewardTrafficReset          RewardKind = "traffic_reset"
	RewardBalanceMultiplier     RewardKind = "balance_multiplier"
	RewardCouponRecurring       RewardKind = "coupon_recurring"
	RewardCouponOnce            RewardKind = "coupon_once"
)

// Reward is the typed payload applied by a lucky-draw prize.
type Reward struct {
	Kind              RewardKind  `json:"kind"`
	TXBDeltaMinor     int64       `json:"txbDeltaMinor,omitempty"`
	CouponID          string      `json:"couponId,omitempty"`
	ExtensionDays     int         `json:"extensionDays,omitempty"`
	Range             *ValueRange `json:"range,omitempty"`
	ResolvedValue     *int64      `json:"resolvedValue,omitempty"`
	ComboID           string      `json:"comboId,omitempty"`
	SquadUUIDs        []string    `json:"squadUuids,omitempty"`
	RenewalPriceMinor int64       `json:"renewalPriceMinor,omitempty"`
	TrafficLimitBytes int64       `json:"trafficLimitBytes,omitempty"`
	IncludeInRenewal  bool        `json:"includeInRenewal,omitempty"`
	DiscountMode      string      `json:"discountMode,omitempty"`
}

// Validate rejects ambiguous or unsafe reward payloads.
func (reward Reward) Validate() error {
	if err := reward.validatePayload(); err != nil {
		return err
	}
	if reward.ResolvedValue != nil {
		return fmt.Errorf("%w: resolved values cannot be configured", ErrInvalidInput)
	}
	switch reward.Kind {
	case RewardNone:
		if reward.TXBDeltaMinor != 0 || reward.CouponID != "" || reward.ExtensionDays != 0 || reward.Range != nil ||
			reward.ComboID != "" || len(reward.SquadUUIDs) > 0 {
			return fmt.Errorf("%w: no-prize reward has a payload", ErrInvalidInput)
		}
	case RewardTXBDelta:
		if reward.Range != nil {
			return reward.Range.Validate(-10000000000, 10000000000, true)
		}
		if reward.TXBDeltaMinor == 0 || reward.TXBDeltaMinor < -10000000000 || reward.TXBDeltaMinor > 10000000000 || reward.CouponID != "" || reward.ExtensionDays != 0 {
			return fmt.Errorf("%w: TXB reward must contain only a non-zero delta", ErrInvalidInput)
		}
	case RewardCouponGrant:
		if strings.TrimSpace(reward.CouponID) == "" || reward.TXBDeltaMinor != 0 || reward.ExtensionDays != 0 {
			return fmt.Errorf("%w: coupon reward must contain only a coupon ID", ErrInvalidInput)
		}
	case RewardSubscriptionExtension:
		if reward.Range != nil {
			return reward.Range.Validate(1, 3650*24, false)
		}
		if reward.ExtensionDays < 1 || reward.ExtensionDays > 3650 || reward.TXBDeltaMinor != 0 || reward.CouponID != "" {
			return fmt.Errorf("%w: extension reward must contain 1 to 3650 days", ErrInvalidInput)
		}
	case RewardEntitlementGrant:
		if strings.TrimSpace(reward.ComboID) == "" || reward.RenewalPriceMinor < 0 || reward.RenewalPriceMinor > 10000000000 ||
			reward.TrafficLimitBytes <= 0 || reward.TrafficLimitBytes > 1000000*(1<<30) ||
			!validRewardSquads(reward.SquadUUIDs, false) {
			return fmt.Errorf("%w: invalid custom entitlement", ErrInvalidInput)
		}
	case RewardSquadAccess:
		if !validRewardSquads(reward.SquadUUIDs, true) {
			return fmt.Errorf("%w: squad reward needs squads", ErrInvalidInput)
		}
	case RewardCoreComboSwitch:
		if strings.TrimSpace(reward.ComboID) == "" {
			return fmt.Errorf("%w: combo reward needs a combo", ErrInvalidInput)
		}
	case RewardTrafficReset:
	case RewardTrafficGrant:
		if reward.Range == nil {
			return fmt.Errorf("%w: traffic reward needs a range", ErrInvalidInput)
		}
		return reward.Range.Validate(-1000000, 1000000, true)
	case RewardBalanceMultiplier:
		if reward.Range == nil {
			return fmt.Errorf("%w: multiplier reward needs a range", ErrInvalidInput)
		}
		value := *reward.Range
		if value.Min < 10000 && value.Max > 10000 {
			if value.PositiveChanceBPS == nil || *value.PositiveChanceBPS < 0 || *value.PositiveChanceBPS > 10000 {
				return fmt.Errorf("%w: multiplier crossing 1x needs gain chance", ErrInvalidInput)
			}
			value.PositiveChanceBPS = nil
		}
		return value.Validate(1, 1000000, false)
	case RewardCouponRecurring, RewardCouponOnce:
		if reward.DiscountMode != "fixed" && reward.DiscountMode != "percent" {
			return fmt.Errorf("%w: invalid coupon discount mode", ErrInvalidInput)
		}
		if reward.Range == nil {
			return fmt.Errorf("%w: coupon reward needs a range", ErrInvalidInput)
		}
		maximum := int64(10000000000)
		if reward.DiscountMode == "percent" {
			maximum = 10000
		}
		return reward.Range.Validate(1, maximum, false)
	default:
		return fmt.Errorf("%w: unsupported reward kind %q", ErrInvalidInput, reward.Kind)
	}
	return nil
}

func validRewardSquads(values []string, required bool) bool {
	if required && len(values) == 0 || len(values) > 100 {
		return false
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}
