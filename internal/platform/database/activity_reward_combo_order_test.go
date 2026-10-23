package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestRecoveryLeavesCustomCombosAwardedAfterCoreChangeUntouched(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC().Add(-2 * time.Hour)
	core := saveTestCombo(t, store, "Original core", 100, 30)
	target := saveTestCombo(t, store, "New core", 200, 45)
	for index, test := range []struct {
		name          string
		priorCustom   bool
		delay         time.Duration
		clearOverride bool
		price         int64
	}{
		{name: "core then custom", delay: time.Minute, price: 54000},
		{name: "core then custom at same timestamp", price: 54000},
		{name: "core then custom within one millisecond", delay: time.Nanosecond, price: 54000},
		{name: "core then free custom", delay: time.Minute},
		{name: "custom then core then newer custom", priorCustom: true, delay: time.Minute, price: 36000},
		{name: "earlier core cannot justify cleared later custom", delay: time.Minute, clearOverride: true, price: 54000},
		{name: "older custom cannot replace cleared newer custom", priorCustom: true, delay: time.Minute, clearOverride: true, price: 36000},
	} {
		t.Run(test.name, func(t *testing.T) {
			user, purchase := createAdminWorkflowPurchase(t, store, int64(31940+index), core, now)
			if test.priorCustom {
				playComboReward(t, store, user.ID, "older custom", activity.Reward{Kind: activity.RewardEntitlementGrant,
					ComboID: core.ID, SquadUUIDs: []string{"older-award"}, RenewalPriceMinor: 63000,
					TrafficLimitBytes: 800 << 30, RolloverMinRemainingBPS: 9999}, now.Add(time.Minute))
			}
			changeAt := now.Add(2 * time.Minute)
			playComboReward(t, store, user.ID, "core change", activity.Reward{Kind: activity.RewardCoreComboSwitch, ComboID: target.ID}, changeAt)
			// Match the core-change target so only ordering and override guards can
			// exclude recovery, rather than an unrelated combo-ID mismatch.
			playComboReward(t, store, user.ID, "latest custom", activity.Reward{Kind: activity.RewardEntitlementGrant,
				ComboID: target.ID, SquadUUIDs: []string{"latest-award"}, RenewalPriceMinor: test.price,
				TrafficLimitBytes: 300 << 30, RolloverMinRemainingBPS: 8000}, changeAt.Add(test.delay))
			if test.clearOverride {
				// Even missing fields do not justify recovery from a core-change
				// reward that predates the most recent custom award.
				result, err := store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_squad_uuids=NULL,
					entitlement_addon_squad_uuids=NULL,entitlement_traffic_limit_bytes=NULL,reward_renewal_price_minor=NULL,
					reward_rollover_min_remaining_bps=NULL,reward_renewal_traffic_limit_bytes=NULL WHERE id=?`, purchase.ID)
				if err != nil {
					t.Fatal(err)
				}
				if count, err := result.RowsAffected(); err != nil || count != 1 {
					t.Fatalf("clear fixture overrides: rows=%d err=%v", count, err)
				}
			}
			before := customComboRecoverySnapshot(t, store, purchase.ID)
			balance := adminWorkflowBalance(t, store, user.ID)
			var jobsBefore int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs`).Scan(&jobsBefore); err != nil {
				t.Fatal(err)
			}
			reapplyRecoveryMigration(t, store, customComboRecoveryMigration)
			if after := customComboRecoverySnapshot(t, store, purchase.ID); after != before {
				t.Fatalf("recovery changed a later custom award:\nbefore=%s\nafter=%s", before, after)
			}
			var audits, jobsAfter int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE target_id=? AND action='activity.custom_combo_recovered'`, purchase.ID).Scan(&audits); err != nil {
				t.Fatal(err)
			}
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs`).Scan(&jobsAfter); err != nil {
				t.Fatal(err)
			}
			if audits != 0 || jobsAfter != jobsBefore || adminWorkflowBalance(t, store, user.ID) != balance {
				t.Fatalf("unexpected recovery side effects: audits=%d jobs=%d -> %d", audits, jobsBefore, jobsAfter)
			}
		})
	}
}

func customComboRecoverySnapshot(t *testing.T, store *Store, purchaseID string) string {
	t.Helper()
	var snapshot string
	err := store.DB().QueryRowContext(context.Background(), `SELECT json_object(
		'combo',combo_id,'squads',entitlement_squad_uuids,'addons',entitlement_addon_squad_uuids,
		'traffic',entitlement_traffic_limit_bytes,'reset',entitlement_reset_strategy,
		'price',reward_renewal_price_minor,'rollover',reward_rollover_min_remaining_bps,
		'renewTraffic',reward_renewal_traffic_limit_bytes,'trafficRenews',reward_traffic_renewal,
		'autoRenew',auto_renew_enabled,'status',status,'from',valid_from,'until',valid_until,
		'charged',charged_txb_minor,'updated',updated_at) FROM purchases WHERE id=?`, purchaseID).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
