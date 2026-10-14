package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestInstantDrawReplayIsBoundToDrawAndMember(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	firstUser := createTestUser(t, store, 31821)
	secondUser := createTestUser(t, store, 31822)
	for _, userID := range []string{firstUser.ID, secondUser.ID} {
		if _, err := store.AdjustBalance(ctx, userID, 1000, "seed:"+userID, "seed", now); err != nil {
			t.Fatal(err)
		}
	}
	input := activity.LuckyDrawInput{Name: "Replay", Kind: "instant", Enabled: true, FeeMinor: 100, ExpectedParticipation: 1,
		Prizes: []activity.PrizeInput{{Name: "Prize", ProbabilityBPS: 10000, Reward: activity.Reward{Kind: activity.RewardTXBDelta, TXBDeltaMinor: 50}}}}
	first, err := store.SaveLuckyDraw(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.SaveLuckyDraw(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	rng := &countingActivityRandom{}
	result, err := store.PlayLuckyDraw(ctx, firstUser.ID, first.ID, "request", rng, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.PlayLuckyDraw(ctx, firstUser.ID, second.ID, "request", rng, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-draw replay: %v", err)
	}
	if _, err = store.PlayLuckyDraw(ctx, firstUser.ID, first.ID, "raffle:reserved", rng, now); !errors.Is(err, activity.ErrInvalidInput) {
		t.Fatalf("reserved settlement key: %v", err)
	}
	replayed, err := store.PlayLuckyDraw(ctx, firstUser.ID, first.ID, "request", rng, now)
	if err != nil || !replayed.Replayed || replayed.ID != result.ID || rng.calls != 1 {
		t.Fatalf("same-draw replay = %+v, %v, rolls %d", replayed, err, rng.calls)
	}
	other, err := store.PlayLuckyDraw(ctx, secondUser.ID, second.ID, "request", rng, now)
	if err != nil || other.Replayed || other.ID == result.ID || rng.calls != 2 {
		t.Fatalf("other member's request = %+v, %v", other, err)
	}
	var balance, results, ledger int
	if err = store.DB().QueryRowContext(ctx, `SELECT txb_minor,
		(SELECT COUNT(*) FROM activity_draw_results WHERE user_id=?),
		(SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind IN ('activity_draw_fee','activity_draw_reward'))
		FROM balances WHERE user_id=?`, firstUser.ID, firstUser.ID, firstUser.ID).Scan(&balance, &results, &ledger); err != nil {
		t.Fatal(err)
	}
	if balance != 950 || results != 1 || ledger != 2 {
		t.Fatalf("first member effects: balance=%d results=%d ledger=%d", balance, results, ledger)
	}
}

func TestDrawSettlementRejectsMalformedStoredRewardBeforeSampling(t *testing.T) {
	t.Parallel()
	rng := &countingActivityRandom{}
	_, err := resolveDrawReward(activity.Reward{Kind: activity.RewardTrafficReset,
		Range: &activity.ValueRange{Min: -1, Max: 1, Distribution: "uniform"}}, rng)
	if !errors.Is(err, activity.ErrInvalidInput) || rng.calls != 0 {
		t.Fatalf("malformed stored reward: %v, rolls %d", err, rng.calls)
	}
}
