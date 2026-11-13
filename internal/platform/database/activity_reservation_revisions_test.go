package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestRaffleReserveAdjustmentTracksEachDefinitionRevision(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	user := createTestUser(t, store, 950022)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	input := auditRaffleInput("join", "join_draw")
	input.Threshold = 2
	input.Prizes[0].Stock = 2
	input.Prizes[0].Reward = activity.Reward{Kind: activity.RewardTXBDelta, TXBDeltaMinor: -100}
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
	if _, matched, err := store.JoinRaffle(ctx, user.ID, -100123, 1, "join", now); err != nil || !matched {
		t.Fatal(err)
	}
	for _, required := range []int64{200, 300, 50} {
		input = draw.LuckyDrawInput
		input.Prizes[0].Reward.TXBDeltaMinor = -required
		draw, err = store.SaveLuckyDraw(ctx, input, now)
		if err != nil {
			t.Fatalf("reserve revision %d: %v", required, err)
		}
		if balance := adminWorkflowBalance(t, store, user.ID); balance != 900-required {
			t.Fatalf("reserve %d changed balance to %d", required, balance)
		}
	}
	var adjustments, total int64
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=$1 AND kind='activity_raffle_reserve_adjust'`, user.ID).Scan(&adjustments); err != nil || adjustments != 3 {
		t.Fatal("missing revision receipt", adjustments, err)
	}
	if err = store.DB().QueryRowContext(ctx, `SELECT SUM(delta_txb_minor) FROM ledger_entries WHERE user_id=$1`, user.ID).Scan(&total); err != nil || total != 850 {
		t.Fatal("ledger conservation failed", total, err)
	}
}
