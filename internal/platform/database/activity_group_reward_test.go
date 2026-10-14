package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestGroupMessageRewardCountsEligibleMessagesAndReplaysAtomically(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 31_300)
	now := time.Date(2026, 8, 8, 16, 30, 0, 0, time.UTC)
	config := activity.GroupMessageRewardConfig{Timezone: "Asia/Shanghai", Threshold: 2, RewardMinor: 125}

	withoutSubscription, err := store.RecordGroupMessage(ctx, user.ID, -100, 1, "2026-08-09", "Asia/Shanghai", config.Threshold, config.RewardMinor, 2, now)
	if err != nil || withoutSubscription.Counted || withoutSubscription.Status.MessageCount != 0 {
		t.Fatalf("message without subscription = (%+v, %v)", withoutSubscription, err)
	}

	combo := saveTestCombo(t, store, "group-reward", 0, 30)
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "group-reward-purchase"}, now)
	if err != nil {
		t.Fatalf("CreatePurchase(): %v", err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='active',valid_from=?,valid_until=? WHERE id=?`, stamp(now.Add(-time.Hour)), stamp(now.Add(24*time.Hour)), purchase.ID); err != nil {
		t.Fatalf("activate test purchase: %v", err)
	}

	first, err := store.RecordGroupMessage(ctx, user.ID, -100, 2, "2026-08-09", "Asia/Shanghai", config.Threshold, config.RewardMinor, 2, now)
	if err != nil || !first.Counted || first.Status.MessageCount != 1 || first.Status.Rewarded {
		t.Fatalf("first message = (%+v, %v)", first, err)
	}
	second, err := store.RecordGroupMessage(ctx, user.ID, -100, 3, "2026-08-09", "Asia/Shanghai", config.Threshold, config.RewardMinor, 2, now)
	if err != nil || !second.Counted || second.Status.MessageCount != 2 || !second.Status.Rewarded {
		t.Fatalf("threshold message = (%+v, %v)", second, err)
	}
	replay, err := store.RecordGroupMessage(ctx, user.ID, -100, 3, "2026-08-09", "Asia/Shanghai", config.Threshold, config.RewardMinor, 2, now.Add(time.Minute))
	if err != nil || !replay.Replayed || replay.Status.MessageCount != 2 {
		t.Fatalf("duplicate message = (%+v, %v)", replay, err)
	}
	status, err := store.GroupMessageRewardStatus(ctx, user.ID, "2026-08-09", config.Threshold, config.RewardMinor)
	if err != nil || !status.Rewarded || status.MessageCount != 2 {
		t.Fatalf("status = (%+v, %v)", status, err)
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "125" {
		t.Fatalf("reward balance = (%+v, %v), want 125", balance, err)
	}
	var rewardLedgerCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind='activity_group_message_reward'`, user.ID).Scan(&rewardLedgerCount); err != nil || rewardLedgerCount != 1 {
		t.Fatalf("reward ledger count = (%d, %v), want 1", rewardLedgerCount, err)
	}
	assertNotificationCounts(t, store, 1, 1)
}

func TestDeductBalanceRejectsInsufficientFunds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 31_301)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "deduct-seed", "seed", time.Now()); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}
	if _, err := store.DeductBalance(ctx, user.ID, 250, "deduct-1", "test deduction", time.Now()); err != nil {
		t.Fatalf("DeductBalance(): %v", err)
	}
	if _, err := store.DeductBalance(ctx, user.ID, 250, "deduct-1", "test deduction", time.Now()); err != nil {
		t.Fatalf("DeductBalance(replay): %v", err)
	}
	if _, err := store.DeductBalance(ctx, user.ID, 300, "deduct-2", "test deduction", time.Now()); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("insufficient deduction error = %v, want %v", err, ErrInsufficientBalance)
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "250" {
		t.Fatalf("balance = (%+v, %v), want 250", balance, err)
	}
}
