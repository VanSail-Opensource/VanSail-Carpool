package database

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestRaffleAccountingReplayAndAtomicSettlement(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	user := createTestUser(t, store, 31841)
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	input := auditRaffleInput("join", "join_draw")
	input.Threshold = 2
	input.Prizes = append(input.Prizes, activity.PrizeInput{Name: "Loss", Stock: 1,
		Reward: activity.Reward{Kind: activity.RewardTXBDelta, TXBDeltaMinor: -250}})
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
	// Concurrent delivery of the same Telegram message must sell exactly one seat.
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			if _, matched, joinErr := store.JoinRaffle(ctx, user.ID, -100123, 1, "join", now); joinErr != nil || !matched {
				t.Errorf("join replay: matched=%t, %v", matched, joinErr)
			}
		})
	}
	group.Wait()
	if receipt, matched, err := store.JoinRaffle(ctx, user.ID, -100123, 2, "join", now); err != nil || !matched || receipt.Seats != 2 {
		t.Fatalf("last seat = %+v, matched=%t, %v", receipt, matched, err)
	}
	randomFailure := errors.New("random source unavailable")
	if _, err = store.SettleRaffle(ctx, draw.ID, fixedActivityRandom{err: randomFailure}, now); !errors.Is(err, randomFailure) {
		t.Fatalf("failed settlement = %v", err)
	}
	var balance, count int64
	if err = store.DB().QueryRowContext(ctx, `SELECT txb_minor FROM balances WHERE user_id=?`, user.ID).Scan(&balance); err != nil || balance != 300 {
		t.Fatalf("held balance = %d, %v", balance, err)
	}
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_draw_results WHERE draw_id=?`, draw.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial results after failure = %d, %v", count, err)
	}
	settled, err := store.SettleRaffle(ctx, draw.ID, fixedActivityRandom{}, now)
	if err != nil || len(settled.Results) != 2 {
		t.Fatalf("settlement = %+v, %v", settled, err)
	}
	if _, err = store.SettleRaffle(ctx, draw.ID, fixedActivityRandom{}, now); err != nil {
		t.Fatal(err)
	}
	if err = store.DB().QueryRowContext(ctx, `SELECT txb_minor FROM balances WHERE user_id=?`, user.ID).Scan(&balance); err != nil || balance != 550 {
		t.Fatalf("settled balance = %d, %v, want 1000 - 200 fees - 250 loss", balance, err)
	}
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(DISTINCT prize_id) FROM activity_draw_results WHERE draw_id=?`, draw.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("distributed stock = %d, %v", count, err)
	}
	var ledgerSum int64
	if err = store.DB().QueryRowContext(ctx, `SELECT SUM(delta_txb_minor) FROM ledger_entries WHERE user_id=?`, user.ID).Scan(&ledgerSum); err != nil || ledgerSum != balance {
		t.Fatalf("ledger sum = %d, balance %d, %v", ledgerSum, balance, err)
	}
}
