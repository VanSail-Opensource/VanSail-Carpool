package activity

import (
	"errors"
	"testing"
)

func TestRewardRejectsAmbiguousAndUnsafePayloads(t *testing.T) {
	t.Parallel()
	valueRange := &ValueRange{Min: 1, Max: 10, Distribution: "uniform"}
	for _, test := range []struct {
		name   string
		reward Reward
	}{
		{"retired reset without payload", Reward{Kind: RewardTrafficReset}},
		{"reset with unchecked signed range", Reward{Kind: RewardTrafficReset, Range: &ValueRange{Min: -1, Max: 1}}},
		{"coupon with range", Reward{Kind: RewardCouponGrant, CouponID: "coupon", Range: valueRange}},
		{"fixed and random TXB", Reward{Kind: RewardTXBDelta, TXBDeltaMinor: 1, Range: valueRange}},
		{"fixed and random extension", Reward{Kind: RewardSubscriptionExtension, ExtensionDays: 1, Range: valueRange}},
		{"no prize with renewal price", Reward{Kind: RewardNone, RenewalPriceMinor: 1}},
		{"money with combo", Reward{Kind: RewardTXBDelta, Range: valueRange, ComboID: "combo"}},
		{"traffic with coupon mode", Reward{Kind: RewardTrafficGrant, Range: valueRange, DiscountMode: "fixed"}},
		{"excessive fixed loss", Reward{Kind: RewardTXBDelta, TXBDeltaMinor: -10000000001}},
		{"excessive fixed gain", Reward{Kind: RewardTXBDelta, TXBDeltaMinor: 10000000001}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.reward.Validate(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}

func TestRewardAcceptsTypedPayloads(t *testing.T) {
	t.Parallel()
	valueRange := &ValueRange{Min: 1, Max: 10, Distribution: "uniform"}
	for _, reward := range []Reward{
		{Kind: RewardNone},
		{Kind: RewardTXBDelta, TXBDeltaMinor: -10000000000},
		{Kind: RewardTXBDelta, TXBDeltaMinor: 10000000000},
		{Kind: RewardTXBDelta, Range: valueRange},
		{Kind: RewardCouponGrant, CouponID: "coupon"},
		{Kind: RewardSubscriptionExtension, Range: valueRange},
		{Kind: RewardSubscriptionExtension, ExtensionDays: 1},
		{Kind: RewardEntitlementGrant, ComboID: "combo", TrafficLimitBytes: 1 << 30, RenewalPriceMinor: 100, SquadUUIDs: []string{"squad"}},
		{Kind: RewardSquadAccess, SquadUUIDs: []string{"squad"}},
		{Kind: RewardCoreComboSwitch, ComboID: "combo"},
		{Kind: RewardTrafficGrant, Range: valueRange, IncludeInRenewal: true},
		{Kind: RewardBalanceMultiplier, Range: valueRange},
		{Kind: RewardCouponOnce, Range: valueRange, DiscountMode: "fixed"},
		{Kind: RewardCouponRecurring, Range: valueRange, DiscountMode: "percent"},
	} {
		if err := reward.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v", reward, err)
		}
	}
}
