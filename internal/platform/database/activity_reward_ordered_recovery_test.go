package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

const orderedCustomRecoveryMigration = "055_restore_ordered_custom_combo_rewards.sql"

func TestOrderedCustomComboRecoveryFromSettledRaffleTickets(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC().Add(-4 * time.Hour)
	core := saveTestCombo(t, store, "Original core", 100, 30)
	target, err := store.SaveCombo(ctx, ComboInput{Name: "New core", PriceTXBMinor: 80000, ValidityDays: 45,
		TrafficLimitBytes: 800 << 30, ResetStrategy: "MONTH_ROLLING", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	standard := activity.Reward{Kind: activity.RewardEntitlementGrant, ComboID: target.ID, SquadUUIDs: []string{"awarded"},
		RenewalPriceMinor: 54000, TrafficLimitBytes: 500 << 30, RolloverMinRemainingBPS: 9999}
	lite := standard
	lite.RenewalPriceMinor, lite.TrafficLimitBytes = 36000, 300<<30
	free := standard
	free.RenewalPriceMinor = 0
	change := activity.Reward{Kind: activity.RewardCoreComboSwitch, ComboID: target.ID}
	temporary := activity.Reward{Kind: activity.RewardTrafficGrant, Range: &activity.ValueRange{Min: 10, Max: 10, Distribution: "uniform"}}
	recurring := activity.Reward{Kind: activity.RewardTrafficGrant, Range: &activity.ValueRange{Min: 20, Max: 20, Distribution: "uniform"}, IncludeInRenewal: true}
	for index, test := range []struct {
		name                    string
		rewards                 []activity.Reward
		recover                 bool
		price, current, renewal int64
	}{
		{"retry damage missed by migration 054", []activity.Reward{standard, change}, true, 54000, 500, 500},
		{"latest of two custom prizes", []activity.Reward{standard, lite, change}, true, 36000, 300, 300},
		{"pending rollover metadata", []activity.Reward{standard, lite, change}, true, 36000, 300, 300},
		{"temporary before core recurring after", []activity.Reward{standard, temporary, change, recurring}, true, 54000, 530, 520},
		{"recurring before core temporary after", []activity.Reward{standard, recurring, change, temporary}, true, 54000, 530, 520},
		{"traffic before core", []activity.Reward{standard, recurring, change}, true, 54000, 520, 520},
		{"newer custom remains authoritative", []activity.Reward{standard, change, lite}, false, 36000, 300, 300},
		{"core before custom remains untouched", []activity.Reward{change, standard}, false, 54000, 500, 500},
		{"free custom with traffic", []activity.Reward{free, recurring, change}, true, 0, 520, 520},
		{"administrator edit", []activity.Reward{standard, lite, change}, false, 0, 0, 0},
		{"unexplained traffic override", []activity.Reward{standard, change, recurring}, false, 0, 0, 0},
		{"missing ticket linkage", []activity.Reward{standard, lite, change}, false, 0, 0, 0},
		{"unrelated user audit", []activity.Reward{standard, recurring, change}, true, 54000, 520, 520},
	} {
		t.Run(test.name, func(t *testing.T) {
			user, purchase := createAdminWorkflowPurchase(t, store, int64(32000+index), core, now)
			if err := store.SetAutoRenewal(ctx, user.ID, purchase.ID, false, now); err != nil {
				t.Fatal(err)
			}
			results := settleRecoveryRaffle(t, store, user.ID, -900000-int64(index), now, test.rewards)
			reproduceLegacyCoreRewards(t, store, purchase.ID, results)
			switch test.name {
			case "pending rollover metadata":
				seedPendingRewardRollover(t, store, purchase.ID, now.Add(2*time.Hour))
			case "administrator edit":
				if err := store.AppendAudit(ctx, &user.ID, "entitlement.edit", "purchase", purchase.ID, "{}", now.Add(2*time.Hour)); err != nil {
					t.Fatal(err)
				}
			case "unrelated user audit":
				if err := store.AppendAudit(ctx, &user.ID, "balance.adjust", "user", user.ID, "{}", now.Add(2*time.Hour)); err != nil {
					t.Fatal(err)
				}
			case "unexplained traffic override":
				if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_traffic_limit_bytes=123 WHERE id=?`, purchase.ID); err != nil {
					t.Fatal(err)
				}
			case "missing ticket linkage":
				if _, err := store.DB().ExecContext(ctx, `UPDATE activity_raffle_tickets SET result_id=NULL WHERE result_id=?`, results[0].ID); err != nil {
					t.Fatal(err)
				}
			}
			before := customComboRecoverySnapshot(t, store, purchase.ID)
			balance := adminWorkflowBalance(t, store, user.ID)
			if test.name != "retry damage missed by migration 054" {
				reapplyRecoveryMigration(t, store, customComboRecoveryMigration)
				if after := customComboRecoverySnapshot(t, store, purchase.ID); after != before {
					t.Fatal("fixture should be excluded by the original migration")
				}
			}
			for range 2 {
				reapplyRecoveryMigration(t, store, orderedCustomRecoveryMigration)
				if !test.recover {
					if after := customComboRecoverySnapshot(t, store, purchase.ID); after != before {
						t.Fatalf("protected purchase changed: before=%s after=%s", before, after)
					}
				} else {
					plan, err := store.AutoRenewalPlan(ctx, user.ID, purchase.ID, time.Now().UTC())
					if err != nil || plan.Combo.ID != target.ID || plan.NetMinor != test.price || plan.trafficLimitOverride == nil || *plan.trafficLimitOverride != test.renewal<<30 || plan.Purchase.AutoRenewEnabled {
						t.Fatalf("restored renewal = (%+v, %v)", plan, err)
					}
					current, err := store.PurchaseByID(ctx, purchase.ID)
					if err != nil || current.TrafficLimitBytes != test.current<<30 || len(current.SquadUUIDs) != 1 || current.SquadUUIDs[0] != "awarded" || !current.ValidUntil.Equal(purchase.ValidUntil) {
						t.Fatalf("restored entitlement = (%+v, %v)", current, err)
					}
					if test.name == "pending rollover metadata" {
						rollover, err := store.RolloverByPurchase(ctx, purchase.ID)
						if err != nil || rollover.TrafficLimitBytes != test.current<<30 || rollover.MinimumRemainingBPS != 9999 {
							t.Fatalf("restored rollover = (%+v, %v)", rollover, err)
						}
					}
				}
				var audits, jobs int
				if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE target_id=? AND action='activity.custom_combo_recovered'`, purchase.ID).Scan(&audits); err != nil {
					t.Fatal(err)
				}
				if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='remna_sync_user' AND json_extract(payload,'$.userId')=? AND status IN ('pending','processing')`, user.ID).Scan(&jobs); err != nil {
					t.Fatal(err)
				}
				if audits != boolInt(test.recover) || jobs != 1 || adminWorkflowBalance(t, store, user.ID) != balance {
					t.Fatalf("recovery side effects: audits=%d jobs=%d", audits, jobs)
				}
			}
		})
	}
}
