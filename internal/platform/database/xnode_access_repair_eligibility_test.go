package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestAccessRepairUsesAccountStatusAndPreservesBlockedBenefits(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 13, 14, 0, 0, 0, time.UTC)
	// The authoritative lifecycle state applies even if retained onboarding
	// history says complete. Blocking the bot alone does not revoke benefits.
	_, err := store.DB().ExecContext(ctx, `INSERT INTO users
		(id,telegram_id,onboarding_state,account_status,bot_blocked,created_at,updated_at)
		VALUES('purged',92001,'complete','purged',1,$1,$1),
		('purging',92002,'complete','purging',1,$1,$1),
		('active-blocked',92003,'complete','active',1,$1,$1)`, stamp(now))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB().ExecContext(ctx, `INSERT INTO users
		(id,telegram_id,onboarding_state,created_at,updated_at) VALUES('z-live',92200,'complete',$1,$1)`, stamp(now))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := store.EnqueueXNodeAccessRepairs(ctx, now); err != nil {
			t.Fatal(err)
		}
	}
	var all, live, blocked int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE payload::jsonb->>'userId'='z-live'),
		COUNT(*) FILTER(WHERE payload::jsonb->>'userId'='active-blocked')
		FROM outbox_jobs WHERE kind=$1`, xnode.AccessRepairJob).Scan(&all, &live, &blocked); err != nil || all != 2 || live != 1 || blocked != 1 {
		t.Fatalf("repair jobs = %d total, %d live, %d blocked active: %v", all, live, blocked, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status='active',bot_blocked=0 WHERE id='purging'`); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueXNodeAccessRepairs(ctx, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var restored int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=$1 AND payload::jsonb->>'userId'='purging'`, xnode.AccessRepairJob).Scan(&restored); err != nil || restored != 1 {
		t.Fatalf("restored subject repair jobs = %d: %v", restored, err)
	}
}
