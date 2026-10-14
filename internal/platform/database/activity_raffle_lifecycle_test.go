package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func auditRaffleInput(keyword, command string) activity.LuckyDrawInput {
	return activity.LuckyDrawInput{Name: "Lifecycle raffle", Kind: "raffle", FeeMinor: 100,
		Threshold: 1, Keyword: keyword, Command: command,
		Prizes: []activity.PrizeInput{{Name: "Seat", Stock: 1, Reward: activity.Reward{Kind: activity.RewardNone}}}}
}

func TestRafflePublishingFreezesTermsAndCleansUpLateMessage(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	actor := createTestUser(t, store, 31811)
	draw, err := store.SaveLuckyDraw(ctx, auditRaffleInput("join", "join_draw"), now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.PublishRaffle(ctx, draw.ID, -100123, now); err != nil {
		t.Fatal(err)
	}
	changed := draw.LuckyDrawInput
	changed.FeeMinor = 200
	if _, err = store.SaveLuckyDraw(ctx, changed, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("edit while publishing: %v", err)
	}
	if err = store.DeleteLuckyDraw(ctx, actor.ID, draw.ID, now); err != nil {
		t.Fatal(err)
	}
	// The original delete job may have read no announcement before confirmation.
	if _, err = store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='processing' WHERE kind='draw_telegram_delete'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = store.ConfirmRafflePublished(ctx, draw.ID, 42, now); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := store.LuckyDrawByID(ctx, draw.ID)
	if err != nil || stored.Status != "cancelled" || stored.AnnouncementMessageID != 42 || stored.FeeMinor != 100 {
		t.Fatalf("cancelled publication = %+v, %v", stored, err)
	}
	var cleanup int
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='draw_telegram_delete'
		AND status='pending' AND json_extract(payload,'$.messageId')=42`).Scan(&cleanup); err != nil || cleanup != 1 {
		t.Fatalf("late cleanup jobs = %d, %v", cleanup, err)
	}
	if _, err = store.SettleRaffle(ctx, draw.ID, fixedActivityRandom{}, now); err != nil {
		t.Fatalf("stale settlement after cancellation: %v", err)
	}
}

func TestSettlingRaffleReservesTriggers(t *testing.T) {
	for _, field := range []string{"keyword", "command"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
			first, err := store.SaveLuckyDraw(ctx, auditRaffleInput("join", "join_draw"), now)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.PublishRaffle(ctx, first.ID, -100123, now); err != nil {
				t.Fatal(err)
			}
			if err = store.ConfirmRafflePublished(ctx, first.ID, 42, now); err != nil {
				t.Fatal(err)
			}
			if _, err = store.DB().ExecContext(ctx, `UPDATE activity_lucky_draws SET status='settling' WHERE id=?`, first.ID); err != nil {
				t.Fatal(err)
			}
			input := auditRaffleInput("other", "other_draw")
			if field == "keyword" {
				input.Keyword = "JOIN"
			} else {
				input.Command = first.Command
			}
			second, err := store.SaveLuckyDraw(ctx, input, now)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.PublishRaffle(ctx, second.ID, -100123, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("publish overlapping %s: %v", field, err)
			}
			input.ID, input.Keyword, input.Command = second.ID, "other", "other_draw"
			second, err = store.SaveLuckyDraw(ctx, input, now)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.PublishRaffle(ctx, second.ID, -100123, now); err != nil {
				t.Fatal(err)
			}
			if err = store.ConfirmRafflePublished(ctx, second.ID, 43, now); err != nil {
				t.Fatal(err)
			}
			if field == "keyword" {
				input.Keyword = "JOIN"
			} else {
				input.Command = first.Command
			}
			if _, err = store.SaveLuckyDraw(ctx, input, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("edit overlapping %s: %v", field, err)
			}
		})
	}
}
