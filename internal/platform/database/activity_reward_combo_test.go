package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func playComboReward(t *testing.T, store *Store, userID, key string, reward activity.Reward, now time.Time) activity.DrawResult {
	t.Helper()
	ctx := context.Background()
	draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{Name: key, Kind: "instant", Enabled: true,
		FeeMinor: 1, ExpectedParticipation: 1, Prizes: []activity.PrizeInput{{Name: key, ProbabilityBPS: 10000, Reward: reward}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.PlayLuckyDraw(ctx, userID, draw.ID, key, fixedActivityRandom{}, now)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCoreChangePreservesCustomTermsAndTrafficRewardLifetimes(t *testing.T) {
	t.Parallel()
	for _, price := range []int64{0, 54_000} {
		t.Run(fmt.Sprint(price), func(t *testing.T) {
			t.Parallel()
			ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
			core := saveTestCombo(t, store, "Original core", 100, 30)
			target := saveTestCombo(t, store, "New core", 200, 45)
			user, purchase := createAdminWorkflowPurchase(t, store, 31901, core, now)
			seedPendingRewardRollover(t, store, purchase.ID, now)
			playComboReward(t, store, user.ID, "custom", activity.Reward{Kind: activity.RewardEntitlementGrant,
				ComboID: core.ID, SquadUUIDs: []string{"awarded"}, RenewalPriceMinor: price,
				TrafficLimitBytes: 500 << 30, RolloverMinRemainingBPS: 9999}, now)
			playComboReward(t, store, user.ID, "recurring traffic", activity.Reward{Kind: activity.RewardTrafficGrant,
				Range: &activity.ValueRange{Min: 20, Max: 20, Distribution: "uniform"}, IncludeInRenewal: true}, now)
			playComboReward(t, store, user.ID, "temporary traffic", activity.Reward{Kind: activity.RewardTrafficGrant,
				Range: &activity.ValueRange{Min: 10, Max: 10, Distribution: "uniform"}}, now)
			playComboReward(t, store, user.ID, "core change", activity.Reward{Kind: activity.RewardCoreComboSwitch, ComboID: target.ID}, now)
			current, err := store.PurchaseByID(ctx, purchase.ID)
			if err != nil || current.ComboID != target.ID || current.TrafficLimitBytes != 530<<30 || len(current.SquadUUIDs) != 1 || current.SquadUUIDs[0] != "awarded" {
				t.Fatalf("current custom combo = (%+v, %v)", current, err)
			}
			plan, err := store.AutoRenewalPlan(ctx, user.ID, purchase.ID, now)
			if err != nil || plan.NetMinor != price || plan.trafficLimitOverride == nil || *plan.trafficLimitOverride != 520<<30 || plan.rewardRolloverBPS == nil || *plan.rewardRolloverBPS != 9999 {
				t.Fatalf("custom renewal plan = (%+v, %v)", plan, err)
			}
			if !current.ValidUntil.Equal(purchase.ValidUntil) || !plan.NextCycleEndsAt.Equal(purchase.ValidUntil.AddDate(0, 0, 45)) {
				t.Fatal("core change moved the current expiry or ignored the new renewal cadence")
			}
			var traffic, threshold int64
			if err := store.DB().QueryRowContext(ctx, `SELECT traffic_limit_bytes,minimum_remaining_bps FROM purchase_rollovers WHERE purchase_id=?`, purchase.ID).Scan(&traffic, &threshold); err != nil || traffic != 530<<30 || threshold != 9999 {
				t.Fatalf("rollover metadata = (%d, %d, %v)", traffic, threshold, err)
			}
		})
	}
}

// A pending rollover snapshot can survive a term extension. Create it explicitly:
// the normal purchase constructor does not create rollovers before expiry.
func seedPendingRewardRollover(t *testing.T, store *Store, purchaseID string, now time.Time) {
	t.Helper()
	result, err := store.DB().ExecContext(context.Background(), `INSERT INTO purchase_rollovers
		(purchase_id,status,traffic_limit_bytes,minimum_remaining_bps,net_paid_txb_minor,created_at,updated_at)
		SELECT p.id,'pending',COALESCE(p.entitlement_traffic_limit_bytes,c.traffic_limit_bytes),
		COALESCE(p.reward_rollover_min_remaining_bps,c.rollover_min_remaining_bps),p.charged_txb_minor,?,?
		FROM purchases p JOIN combos c ON c.id=p.combo_id WHERE p.id=?`, stamp(now), stamp(now), purchaseID)
	if err != nil {
		t.Fatal(err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		t.Fatalf("create pending rollover fixture: rows=%d err=%v", count, err)
	}
}

func TestOrdinaryCoreChangeStillAdoptsTargetDefaults(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	core := saveTestCombo(t, store, "Original core", 100, 30)
	target := saveTestCombo(t, store, "New core", 200, 45)
	user, purchase := createAdminWorkflowPurchase(t, store, 31902, core, now)
	playComboReward(t, store, user.ID, "core change", activity.Reward{Kind: activity.RewardCoreComboSwitch, ComboID: target.ID}, now)
	plan, err := store.AutoRenewalPlan(ctx, user.ID, purchase.ID, now)
	if err != nil || plan.Combo.ID != target.ID || plan.NetMinor != target.PriceTXBMinor || plan.rewardRenewalPrice != nil || plan.trafficLimitOverride != nil {
		t.Fatalf("ordinary core renewal = (%+v, %v)", plan, err)
	}
	// A later full custom prize still replaces the entire entitlement.
	playComboReward(t, store, user.ID, "later custom", activity.Reward{Kind: activity.RewardEntitlementGrant,
		ComboID: core.ID, SquadUUIDs: []string{"awarded"}, RenewalPriceMinor: 36000,
		TrafficLimitBytes: 300 << 30, RolloverMinRemainingBPS: 9999}, now)
	plan, err = store.AutoRenewalPlan(ctx, user.ID, purchase.ID, now)
	if err != nil || plan.Combo.ID != core.ID || plan.NetMinor != 36000 || plan.trafficLimitOverride == nil || *plan.trafficLimitOverride != 300<<30 {
		t.Fatalf("later custom renewal = (%+v, %v)", plan, err)
	}
}
