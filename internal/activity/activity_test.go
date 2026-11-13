package activity

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestRewardValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reward  Reward
		wantErr bool
	}{
		{name: "no prize", reward: Reward{Kind: RewardNone}},
		{name: "positive TXB", reward: Reward{Kind: RewardTXBDelta, TXBDeltaMinor: 250}},
		{name: "negative TXB", reward: Reward{Kind: RewardTXBDelta, TXBDeltaMinor: -250}},
		{name: "minimum integer TXB", reward: Reward{Kind: RewardTXBDelta, TXBDeltaMinor: math.MinInt64}, wantErr: true},
		{name: "coupon", reward: Reward{Kind: RewardCouponGrant, CouponID: "coupon-1"}},
		{name: "extension", reward: Reward{Kind: RewardSubscriptionExtension, ExtensionDays: 7}},
		{name: "zero TXB", reward: Reward{Kind: RewardTXBDelta}, wantErr: true},
		{name: "mixed payload", reward: Reward{Kind: RewardCouponGrant, CouponID: "coupon-1", ExtensionDays: 1}, wantErr: true},
		{name: "unknown", reward: Reward{Kind: "mystery"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.reward.Validate()
			if test.wantErr != errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Validate() error = %v, want invalid=%t", err, test.wantErr)
			}
		})
	}
}

func TestLuckyDrawMaximumPrizeDeduction(t *testing.T) {
	t.Parallel()

	input := LuckyDrawInput{Prizes: []PrizeInput{
		{Reward: Reward{Kind: RewardTXBDelta, TXBDeltaMinor: -100}},
		{Reward: Reward{Kind: RewardTXBDelta, TXBDeltaMinor: 500}},
		{Reward: Reward{Kind: RewardTXBDelta, TXBDeltaMinor: -350}},
	}}
	if got := input.MaximumPrizeDeduction(); got != 350 {
		t.Fatalf("MaximumPrizeDeduction() = %d, want 350", got)
	}
}

func TestLuckyDrawMaximumPrizeDeductionDoesNotOverflow(t *testing.T) {
	t.Parallel()

	input := LuckyDrawInput{Prizes: []PrizeInput{{Reward: Reward{Kind: RewardTXBDelta, TXBDeltaMinor: math.MinInt64}}}}
	if got := input.MaximumPrizeDeduction(); got != math.MaxInt64 {
		t.Fatalf("MaximumPrizeDeduction() = %d, want %d", got, int64(math.MaxInt64))
	}
	input.Prizes[0].Reward = Reward{Kind: RewardTXBDelta, Range: &ValueRange{Min: math.MinInt64, Max: -1}}
	if got := input.MaximumPrizeDeduction(); got != math.MaxInt64 {
		t.Fatalf("MaximumPrizeDeduction() for range = %d, want %d", got, int64(math.MaxInt64))
	}
}

func validInstantDraw(description string) LuckyDrawInput {
	return LuckyDrawInput{Name: "Draw", Description: description, Kind: "instant", Status: "draft",
		FeeMinor: 100, ExpectedParticipation: 50,
		Prizes: []PrizeInput{{Name: "None", ProbabilityBPS: 10_000, Reward: Reward{Kind: RewardNone}}}}
}

func TestActivityDescriptionsAreBounded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		validate func() error
		wantErr  bool
	}{
		{
			name: "game accepts bounded description",
			validate: func() error {
				return (GameInput{Name: "Coin", Icon: "coin", Description: strings.Repeat("g", 4_000), WinChanceBPS: 5_000,
					MinimumStakeMinor: 1, MaximumStakeMinor: 10, ReturnMultiplierBPS: 20_000}).Validate()
			},
		},
		{
			name: "game rejects oversized description",
			validate: func() error {
				return (GameInput{Name: "Coin", Icon: "coin", Description: strings.Repeat("g", 4_001), WinChanceBPS: 5_000,
					MinimumStakeMinor: 1, MaximumStakeMinor: 10, ReturnMultiplierBPS: 20_000}).Validate()
			},
			wantErr: true,
		},
		{
			name: "draw accepts bounded description",
			validate: func() error {
				return validInstantDraw(strings.Repeat("d", 300)).Validate()
			},
		},
		{
			name: "draw rejects oversized description",
			validate: func() error {
				return validInstantDraw(strings.Repeat("d", 301)).Validate()
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.validate()
			if got := errors.Is(err, ErrInvalidInput); got != test.wantErr {
				t.Fatalf("Validate() error = %v, invalid=%t, want %t", err, got, test.wantErr)
			}
		})
	}
}
