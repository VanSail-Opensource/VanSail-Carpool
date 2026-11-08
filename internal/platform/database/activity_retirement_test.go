package database

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"sync"
	"testing"
	"time"
)

func TestRetiredResetRaffleRefundsFeeAndReservationOnce(t *testing.T) {
	s, ctx, now := newTestStore(t), context.Background(), time.Now().UTC()
	user := createTestUser(t, s, 31991)
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdjustBalance(ctx, user.ID, 1000, "retirement-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	input := auditRaffleInput("join_retired", "join_retired_draw")
	input.Threshold = 2
	input.Prizes = append(input.Prizes, activity.PrizeInput{Name: "Loss", Stock: 1, Reward: activity.Reward{Kind: activity.RewardTXBDelta, TXBDeltaMinor: -250}})
	draw, err := s.SaveLuckyDraw(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.PublishRaffle(ctx, draw.ID, -100123, now); err != nil {
		t.Fatal(err)
	}
	if err = s.ConfirmRafflePublished(ctx, draw.ID, 42, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.JoinRaffle(ctx, user.ID, -100123, 1, "join_retired", now); err != nil {
		t.Fatal(err)
	}
	// Model a definition sold by the previous release. New configuration rejects it.
	if _, err = s.db.ExecContext(ctx, `UPDATE activity_lucky_prizes SET reward_payload='{"kind":"traffic_reset"}' WHERE id=$1`, draw.Prizes[0].ID); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := range 3 {
		group.Go(func() {
			if _, _, err := s.JoinRaffle(ctx, user.ID, -100123, int64(i+2), "join_retired", now); err != nil && !errors.Is(err, ErrConflict) {
				t.Errorf("retired join: %v", err)
			}
		})
	}
	group.Go(func() {
		if _, err := s.SettleRaffle(ctx, draw.ID, fixedActivityRandom{}, now); err != nil {
			t.Errorf("retired settlement: %v", err)
		}
	})
	group.Wait()
	balance, err := s.Balance(ctx, user.ID)
	if err != nil || balance.MinorInt64() != 1000 {
		t.Fatal(balance, err)
	}
	var refunds int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=$1 AND kind='activity_raffle_refund'`, user.ID).Scan(&refunds); err != nil || refunds != 1 {
		t.Fatal(refunds, err)
	}
	stored, err := s.LuckyDrawByID(ctx, draw.ID)
	if err != nil || stored.Status != "cancelled" {
		t.Fatal(stored, err)
	}
}

func TestRetiredResetDrawHiddenAndRejectedWithoutDebit(t *testing.T) {
	s, ctx, now := newTestStore(t), context.Background(), time.Now().UTC()
	user := createTestUser(t, s, 31992)
	if _, err := s.AdjustBalance(ctx, user.ID, 1000, "retired-instant-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	input := activity.LuckyDrawInput{Name: "Old reset", Kind: "instant", Enabled: true, FeeMinor: 100, ExpectedParticipation: 1, Prizes: []activity.PrizeInput{{Name: "Reset", ProbabilityBPS: 10000, Reward: activity.Reward{Kind: activity.RewardTrafficReset}}}}
	if _, err := s.SaveLuckyDraw(ctx, input, now); !errors.Is(err, activity.ErrInvalidInput) {
		t.Fatal("new reset draw accepted", err)
	}
	input.Prizes[0].Reward.Kind = activity.RewardNone
	draw, err := s.SaveLuckyDraw(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE activity_lucky_prizes SET reward_payload='{"kind":"traffic_reset"}' WHERE draw_id=$1`, draw.ID); err != nil {
		t.Fatal(err)
	}
	listed, err := s.ListLuckyDraws(ctx, true)
	if err != nil || len(listed) != 0 {
		t.Fatal(listed, err)
	}
	if _, err = s.PlayLuckyDraw(ctx, user.ID, draw.ID, "new-play", fixedActivityRandom{}, now); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	for range 2 {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err = retireResetDrawsTx(ctx, tx, now, map[string]int64{}); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	balance, err := s.Balance(ctx, user.ID)
	if err != nil || balance.MinorInt64() != 1000 {
		t.Fatal(balance, err)
	}
}
