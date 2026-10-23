package database

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

type recoveryIdentityRandom struct{}

func (recoveryIdentityRandom) Int63n(bound int64) (int64, error) { return bound - 1, nil }

func settleRecoveryRaffle(t *testing.T, store *Store, userID string, chatID int64, now time.Time, rewards []activity.Reward) []activity.DrawResult {
	t.Helper()
	ctx := context.Background()
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=?`, userID); err != nil {
		t.Fatal(err)
	}
	prizes := make([]activity.PrizeInput, 0, len(rewards))
	for index, reward := range rewards {
		prizes = append(prizes, activity.PrizeInput{Name: fmt.Sprint("Prize ", index), Stock: 1, Reward: reward})
	}
	draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{Name: "Recovery raffle", Kind: "raffle",
		FeeMinor: 1, Threshold: len(prizes), Keyword: "recover", Command: "recover_custom", Prizes: prizes}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PublishRaffle(ctx, draw.ID, chatID, now); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmRafflePublished(ctx, draw.ID, 42, now); err != nil {
		t.Fatal(err)
	}
	for index := range prizes {
		if _, matched, err := store.JoinRaffle(ctx, userID, chatID, int64(index+1), "recover", now.Add(time.Duration(index+1)*time.Minute)); err != nil || !matched {
			t.Fatalf("join recovery raffle: matched=%t err=%v", matched, err)
		}
	}
	result, err := store.SettleRaffle(ctx, draw.ID, recoveryIdentityRandom{}, now.Add(time.Hour))
	if err != nil || len(result.Results) != len(rewards) {
		t.Fatalf("settle recovery raffle: results=%d err=%v", len(result.Results), err)
	}
	return result.Results
}

// Build the legacy entitlement state while leaving actual settled results,
// ticket order and financial records unchanged. No draw is charged twice.
func reproduceLegacyCoreRewards(t *testing.T, store *Store, purchaseID string, results []activity.DrawResult) {
	t.Helper()
	ctx := context.Background()
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, result := range results {
		if result.Reward.Kind == activity.RewardCoreComboSwitch {
			_, err = tx.ExecContext(ctx, `UPDATE purchases SET combo_id=?,entitlement_squad_uuids=NULL,
				entitlement_addon_squad_uuids=NULL,entitlement_traffic_limit_bytes=NULL,reward_renewal_price_minor=NULL,
				reward_rollover_min_remaining_bps=NULL,reward_renewal_traffic_limit_bytes=NULL,reward_traffic_renewal=1 WHERE id=?`, result.Reward.ComboID, purchaseID)
		} else {
			value := int64(0)
			if result.Reward.ResolvedValue != nil {
				value = *result.Reward.ResolvedValue
			}
			err = applyDrawEntitlementTx(ctx, tx, result.UserID, result.ID, result.Reward, value, result.CreatedAt)
			if err == nil && result.Reward.Kind == activity.RewardEntitlementGrant {
				// Legacy custom awards inherited cadence rather than snapshotting it.
				_, err = tx.ExecContext(ctx, `UPDATE purchases SET entitlement_reset_strategy=NULL WHERE id=?`, purchaseID)
			}
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// Historical repair intent is replayed against the current native schema.
func reapplyRecoveryMigration(t *testing.T, store *Store, name string) {
	t.Helper()
	raw, err := migrations.ReadFile("migrations/" + name)
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(string(raw), "remna_sync_user", "xnode_sync_user")
	if _, err = store.DB().ExecContext(context.Background(), script); err != nil {
		t.Fatal(err)
	}
}
