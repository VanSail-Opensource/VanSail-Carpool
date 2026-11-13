package activity

import (
	"context"
	"testing"
	"time"
)

type activityAdminFixture struct {
	Store
	actor string
	game  GameInput
	draw  LuckyDrawInput
	id    string
	group int64
	now   time.Time
}

func (f *activityAdminFixture) SaveAdminActivityGame(_ context.Context, actor string, input GameInput, now time.Time) (Game, error) {
	f.actor, f.game, f.now = actor, input, now
	return Game{GameInput: input}, nil
}
func (f *activityAdminFixture) SaveAdminLuckyDraw(_ context.Context, actor string, input LuckyDrawInput, now time.Time) (LuckyDraw, error) {
	f.actor, f.draw, f.now = actor, input, now
	return LuckyDraw{LuckyDrawInput: input}, nil
}
func (f *activityAdminFixture) PublishAdminRaffle(_ context.Context, actor, id string, group int64, now time.Time) error {
	f.actor, f.id, f.group, f.now = actor, id, group, now
	return nil
}

func TestActivityAdminUsesAuditedAdapterWithCurrentActor(t *testing.T) {
	t.Parallel()
	ctx, now := context.Background(), time.Date(2026, 11, 13, 12, 0, 0, 0, time.FixedZone("fixture", 3600))
	fixture := &activityAdminFixture{}
	service := NewService(fixture, nil, func() time.Time { return now })
	game := GameInput{Name: " Coin ", Icon: " coin ", Description: " Terms ", Enabled: true, WinChanceBPS: 5000,
		MinimumStakeMinor: 100, MaximumStakeMinor: 500, ReturnMultiplierBPS: 20000}
	if _, err := service.SaveAdminGame(ctx, "admin", game); err != nil || fixture.actor != "admin" || fixture.game.Name != "Coin" || fixture.game.Icon != "coin" || fixture.game.Description != "Terms" || fixture.now != now.UTC() {
		t.Fatal(fixture, err)
	}
	draw := LuckyDrawInput{Name: " Draw ", Description: " Terms ", Prizes: []PrizeInput{{Name: " Prize ", Reward: Reward{CouponID: " coupon "}}}}
	if _, err := service.SaveAdminDraw(ctx, "admin", draw); err != nil || fixture.actor != "admin" || fixture.draw.Name != "Draw" || fixture.draw.Prizes[0].Name != "Prize" || fixture.draw.Prizes[0].Reward.CouponID != "coupon" {
		t.Fatal(fixture, err)
	}
	if draw.Prizes[0].Name != " Prize " || draw.Prizes[0].Reward.CouponID != " coupon " {
		t.Fatal("normalization mutated caller-owned prizes")
	}
	if err := service.PublishAdminRaffle(ctx, "admin", "draw-id", -100123); err != nil || fixture.actor != "admin" || fixture.id != "draw-id" || fixture.group != -100123 {
		t.Fatal(fixture, err)
	}
	unsupported := NewService(struct{ Store }{}, nil, nil)
	if _, err := unsupported.SaveAdminGame(ctx, "admin", game); err == nil {
		t.Fatal("admin game fell back to raw mutation")
	}
	if _, err := unsupported.SaveAdminDraw(ctx, "admin", draw); err == nil {
		t.Fatal("admin draw fell back to raw mutation")
	}
	if err := unsupported.PublishAdminRaffle(ctx, "admin", "draw-id", -100123); err == nil {
		t.Fatal("admin publish fell back to raw mutation")
	}
}
