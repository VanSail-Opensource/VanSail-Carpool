package activity

import "fmt"

// validatePayload rejects fields from other reward kinds before any range is
// sampled. A configured reward has exactly one effect and one value source.
func (reward Reward) validatePayload() error {
	remaining := reward
	switch reward.Kind {
	case RewardTXBDelta:
		if reward.Range != nil && reward.TXBDeltaMinor != 0 {
			return fmt.Errorf("%w: TXB reward has both a range and a fixed value", ErrInvalidInput)
		}
		remaining.Range, remaining.TXBDeltaMinor = nil, 0
	case RewardCouponGrant:
		remaining.CouponID = ""
	case RewardSubscriptionExtension:
		if reward.Range != nil && reward.ExtensionDays != 0 {
			return fmt.Errorf("%w: extension has both a range and fixed days", ErrInvalidInput)
		}
		remaining.Range, remaining.ExtensionDays = nil, 0
	case RewardEntitlementGrant:
		remaining.ComboID, remaining.SquadUUIDs = "", nil
		remaining.RenewalPriceMinor, remaining.TrafficLimitBytes = 0, 0
	case RewardSquadAccess:
		remaining.SquadUUIDs = nil
	case RewardCoreComboSwitch:
		remaining.ComboID = ""
	case RewardTrafficGrant:
		remaining.Range, remaining.IncludeInRenewal = nil, false
	case RewardBalanceMultiplier:
		remaining.Range = nil
	case RewardCouponRecurring, RewardCouponOnce:
		remaining.Range, remaining.DiscountMode = nil, ""
	}
	if remaining.Range != nil || remaining.TXBDeltaMinor != 0 || remaining.CouponID != "" || remaining.ExtensionDays != 0 ||
		remaining.ComboID != "" || len(remaining.SquadUUIDs) != 0 || remaining.RenewalPriceMinor != 0 ||
		remaining.TrafficLimitBytes != 0 || remaining.IncludeInRenewal || remaining.DiscountMode != "" {
		return fmt.Errorf("%w: fields do not belong to reward kind %q", ErrInvalidInput, reward.Kind)
	}
	return nil
}
