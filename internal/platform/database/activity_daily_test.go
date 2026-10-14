package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestDailyActivityUsesConfiguredTimezoneDate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 31_100)
	now := time.Date(2026, 8, 8, 16, 30, 0, 0, time.UTC) // 2026-08-09 in Asia/Shanghai.
	claimed, err := store.ClaimDailyActivity(ctx, user.ID, "2026-08-09", "Asia/Shanghai", 125, now)
	if err != nil {
		t.Fatalf("ClaimDailyActivity(): %v", err)
	}
	replayed, err := store.ClaimDailyActivity(ctx, user.ID, "2026-08-09", "Asia/Shanghai", 999, now.Add(time.Hour))
	if err != nil || !replayed.AlreadyClaimed || replayed.ID != claimed.ID || replayed.RewardMinor != 125 {
		t.Fatalf("check-in replay = (%+v, %v)", replayed, err)
	}
	if _, err := store.ClaimDailyActivity(ctx, user.ID, "2026-08-08", "Asia/Shanghai", 125, now); !errors.Is(err, activity.ErrInvalidInput) {
		t.Fatalf("wrong local date error = %v, want invalid input", err)
	}
}

func TestDailyActivityRangeChoosesOnceAndReplaysPersistedReward(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 31_102)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	random := &countingActivityRandom{value: 50}
	claimed, err := store.ClaimDailyActivityRange(ctx, user.ID, "2026-08-08", "UTC", 100, 200, random, now)
	if err != nil || claimed.RewardMinor != 150 || random.calls != 1 {
		t.Fatalf("ClaimDailyActivityRange() = (%+v, %v), random calls %d", claimed, err, random.calls)
	}
	random.value = 99
	replayed, err := store.ClaimDailyActivityRange(ctx, user.ID, "2026-08-08", "UTC", 100, 200, random, now.Add(time.Hour))
	if err != nil || !replayed.AlreadyClaimed || replayed.ID != claimed.ID || replayed.RewardMinor != 150 || random.calls != 1 {
		t.Fatalf("replayed range = (%+v, %v), random calls %d", replayed, err, random.calls)
	}
}

func TestDailyActivityRewardCanReduceExistingDebt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 31_101)
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, -500, "activity-debt", "refund debt", now); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}
	claimed, err := store.ClaimDailyActivity(ctx, user.ID, "2026-08-08", "UTC", 125, now)
	if err != nil {
		t.Fatalf("ClaimDailyActivity(): %v", err)
	}
	if claimed.BalanceAfterMinor != -375 {
		t.Fatalf("BalanceAfterMinor = %d, want -375", claimed.BalanceAfterMinor)
	}
	var ledgerCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE kind='activity_daily_checkin' AND reference_id=?`, claimed.ID).Scan(&ledgerCount); err != nil {
		t.Fatalf("count ledger: %v", err)
	}
	if ledgerCount != 1 {
		t.Fatalf("ledger count = %d, want 1", ledgerCount)
	}
}
