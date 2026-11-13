package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

const invalidStoredPrize = `{"kind":"txb_delta","range":{"min":-9223372036854775808,"max":-1,"distribution":"uniform"}}`

func TestInstantDrawValidatesEveryStoredPrizeBeforeSamplingOrFee(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	user := createTestUser(t, store, 950020)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{Name: "Stored definition", Kind: "instant", Enabled: true, FeeMinor: 100, ExpectedParticipation: 1,
		Prizes: []activity.PrizeInput{{Name: "Selected", ProbabilityBPS: 5000, Reward: activity.Reward{Kind: activity.RewardNone}}, {Name: "Other", ProbabilityBPS: 5000, Reward: activity.Reward{Kind: activity.RewardNone}}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE activity_lucky_prizes SET reward_payload=$1 WHERE id=$2`, invalidStoredPrize, draw.Prizes[1].ID); err != nil {
		t.Fatal(err)
	}
	rng := &countingActivityRandom{}
	if _, err = store.PlayLuckyDraw(ctx, user.ID, draw.ID, "bad", rng, now); !errors.Is(err, activity.ErrInvalidInput) {
		t.Fatal("malformed unselected prize accepted", err)
	}
	if rng.calls != 0 {
		t.Fatalf("invalid definition sampled %d times", rng.calls)
	}
	if balance := adminWorkflowBalance(t, store, user.ID); balance != 1000 {
		t.Fatalf("invalid definition charged %d", balance)
	}
	assertRowCount(t, store, "activity_draw_results", 0)
	if _, err = store.DB().ExecContext(ctx, `UPDATE activity_lucky_prizes SET reward_payload='{"kind":"none"}' WHERE id=$1`, draw.Prizes[1].ID); err != nil {
		t.Fatal(err)
	}
	result, err := store.PlayLuckyDraw(ctx, user.ID, draw.ID, "bad", rng, now)
	if err != nil || result.BalanceAfterMinor != 900 {
		t.Fatal("corrected definition did not recover", result, err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE activity_lucky_prizes SET reward_payload=$1 WHERE id=$2`, invalidStoredPrize, draw.Prizes[1].ID); err != nil {
		t.Fatal(err)
	}
	replay, err := store.PlayLuckyDraw(ctx, user.ID, draw.ID, "bad", rng, now)
	if err != nil || !replay.Replayed || replay.ID != result.ID {
		t.Fatal("committed receipt blocked by later corruption", replay, err)
	}
}

func TestRaffleValidatesStoredDefinitionBeforeReservationAndSettlement(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	user := createTestUser(t, store, 950021)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	input := auditRaffleInput("join", "join_draw")
	input.Threshold = 2
	input.Prizes[0].Stock = 2
	draw, err := store.SaveLuckyDraw(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.PublishRaffle(ctx, draw.ID, -100123, now); err != nil {
		t.Fatal(err)
	}
	if err = store.ConfirmRafflePublished(ctx, draw.ID, 42, now); err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE activity_lucky_prizes SET reward_payload=$1 WHERE id=$2`, invalidStoredPrize, draw.Prizes[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, matched, err := store.JoinRaffle(ctx, user.ID, -100123, 1, "join", now); !matched || !errors.Is(err, activity.ErrInvalidInput) {
		t.Fatal("invalid reservation accepted", matched, err)
	}
	if balance := adminWorkflowBalance(t, store, user.ID); balance != 1000 {
		t.Fatal("invalid reservation changed wallet", balance)
	}
	assertRowCount(t, store, "activity_raffle_tickets", 0)
	if _, err = store.DB().ExecContext(ctx, `UPDATE activity_lucky_prizes SET reward_payload='{"kind":"none"}' WHERE id=$1`, draw.Prizes[0].ID); err != nil {
		t.Fatal(err)
	}
	for _, message := range []int64{1, 2} {
		if _, matched, err := store.JoinRaffle(ctx, user.ID, -100123, message, "join", now); err != nil || !matched {
			t.Fatal("valid seat failed", err)
		}
	}
	// Aggregate stock is not a column-level CHECK: reject before constructing slots.
	if _, err = store.DB().ExecContext(ctx, `UPDATE activity_lucky_prizes SET stock=2001 WHERE id=$1`, draw.Prizes[0].ID); err != nil {
		t.Fatal(err)
	}
	rng := &countingActivityRandom{}
	if _, err = store.SettleRaffle(ctx, draw.ID, rng, now); !errors.Is(err, activity.ErrInvalidInput) {
		t.Fatal("invalid settlement accepted", err)
	}
	if rng.calls != 0 || adminWorkflowBalance(t, store, user.ID) != 800 {
		t.Fatal("invalid settlement sampled or changed wallet", rng.calls)
	}
	assertRowCount(t, store, "activity_draw_results", 0)
	if _, err = store.DB().ExecContext(ctx, `UPDATE activity_lucky_prizes SET stock=2 WHERE id=$1`, draw.Prizes[0].ID); err != nil {
		t.Fatal(err)
	}
	if result, err := store.SettleRaffle(ctx, draw.ID, rng, now); err != nil || len(result.Results) != 2 {
		t.Fatal("corrected settlement failed", result, err)
	}
}
