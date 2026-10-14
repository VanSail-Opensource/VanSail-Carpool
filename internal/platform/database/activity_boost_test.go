package database

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestBoostedCheckInSamplesBaseOnceAcrossConcurrentClaims(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 31900)
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	random := &countingActivityRandom{value: 1}
	var group sync.WaitGroup
	results := make(chan activity.DailyCheckIn, 8)
	for index := 0; index < 8; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := store.ClaimBoostedDailyActivityRange(ctx, user.ID, "2026-10-07", "UTC", 100, 102, 3, random, now)
			if err != nil {
				t.Error(err)
				return
			}
			results <- result
		}()
	}
	group.Wait()
	close(results)
	id := ""
	for result := range results {
		if id == "" {
			id = result.ID
		}
		if result.ID != id || result.RewardMinor != 152 {
			t.Fatalf("receipt = %+v", result)
		}
	}
	if random.calls != 1 {
		t.Fatalf("random calls = %d", random.calls)
	}
	replay, err := store.ClaimBoostedDailyActivityRange(ctx, user.ID, "2026-10-07", "UTC", 100, 102, 4, random, now)
	if err != nil || !replay.AlreadyClaimed || replay.RewardMinor != 152 {
		t.Fatalf("changed boosts replay = %+v, %v", replay, err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind='activity_daily_checkin'`, user.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("ledger count = %d, %v", count, err)
	}
}

func TestBoostedCheckInRefusalAndOverflowDoNotCredit(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 31901)
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		base  int64
		count int
		want  error
	}{
		{125, 0, activity.ErrGroupBoostRequired},
		{math.MaxInt64, 3, activity.ErrInvalidInput},
	} {
		_, err := store.ClaimBoostedDailyActivityRange(ctx, user.ID, "2026-10-07", "UTC", test.base, test.base, test.count, nil, now)
		if !errors.Is(err, test.want) {
			t.Fatalf("claim error = %v", err)
		}
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "0" {
		t.Fatalf("balance = %+v, %v", balance, err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_daily_checkins WHERE user_id=?`, user.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("claims = %d, %v", count, err)
	}
}

func createBoostRewardMember(t *testing.T, store *Store, telegramID int64, now time.Time) model.User {
	t.Helper()
	ctx := context.Background()
	user := createTestUser(t, store, telegramID)
	combo := saveTestCombo(t, store, user.ID, 0, 30)
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: user.ID}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='active',valid_from=?,valid_until=? WHERE id=?`, stamp(now.Add(-time.Hour)), stamp(now.Add(24*time.Hour)), purchase.ID); err != nil {
		t.Fatal(err)
	}
	return user
}

func TestGroupProgressRequiresBoostAndPreservesPaidAmount(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	user := createBoostRewardMember(t, store, 31902, now)
	for index, boosts := range []int{0, 1, 0, 3, 4} {
		result, err := store.RecordGroupMessage(ctx, user.ID, -100123, int64(index+1), "2026-10-07", "UTC", 2, 125, boosts, now)
		if err != nil || result.Counted != (boosts > 0) {
			t.Fatalf("message %d = %+v, %v", index, result, err)
		}
		if index < 3 && result.Status.Rewarded {
			t.Fatalf("premature reward at %d", index)
		}
		if index == 2 && result.Status.MessageCount != 1 {
			t.Fatal("unboosted message advanced progress")
		}
		if index >= 3 && result.Status.RewardMinor != 188 {
			t.Fatalf("paid amount changed: %+v", result)
		}
	}
	replay, err := store.RecordGroupMessage(ctx, user.ID, -100123, 1, "2026-10-07", "UTC", 2, 125, 4, now)
	if err != nil || !replay.Replayed || replay.Counted || replay.Status.MessageCount != 3 {
		t.Fatalf("unboosted replay = %+v, %v", replay, err)
	}
	status, err := store.GroupMessageRewardStatus(ctx, user.ID, "2026-10-07", 2, 999)
	if err != nil || status.RewardMinor != 188 {
		t.Fatalf("stored reward = %+v, %v", status, err)
	}
	assertNotificationCounts(t, store, 1, 1)
}

func TestConcurrentGroupMessageReplayCreditsOnce(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	user := createBoostRewardMember(t, store, 31903, now)
	var group sync.WaitGroup
	for index := 0; index < 8; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := store.RecordGroupMessage(ctx, user.ID, -100123, 10, "2026-10-07", "UTC", 1, 125, 3, now); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind='activity_group_message_reward'`, user.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("ledger count = %d, %v", count, err)
	}
}
