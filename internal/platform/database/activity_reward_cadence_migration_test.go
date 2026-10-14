package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

const customCadenceMigration = "056_restore_custom_combo_reset_cadence.sql"

func TestCustomCadenceRecoveryUsesAwardCoreAndPreservesExplicitOverrides(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	for index, test := range []struct {
		name, want string
		recover    bool
	}{
		{"already preserved custom terms", "WEEK", true},
		{"terms restored by 054", "WEEK", true},
		{"terms restored by 055", "WEEK", true},
		{"later custom in same raffle", "MONTH_ROLLING", true},
		{"automatic renewal successor", "WEEK", true},
		{"explicit cadence", "MONTH_ROLLING", false},
		{"administrator edit", "DAY", false},
		{"changed custom terms", "DAY", false},
		{"ambiguous latest custom", "DAY", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Now().UTC().Add(-4 * time.Hour)
			if test.name == "automatic renewal successor" {
				now = time.Now().UTC().AddDate(0, 0, -31)
			}
			core := saveCadenceCombo(t, store, "Award core", "WEEK")
			target := saveCadenceCombo(t, store, "Target core", "DAY")
			monthly := saveCadenceCombo(t, store, "Later award core", "MONTH_ROLLING")
			user, purchase := createAdminWorkflowPurchase(t, store, int64(32120+index), core, now)
			custom := activity.Reward{Kind: activity.RewardEntitlementGrant, ComboID: core.ID, SquadUUIDs: []string{"awarded"},
				RenewalPriceMinor: 500, TrafficLimitBytes: 500 << 30, RolloverMinRemainingBPS: 9999}
			change := activity.Reward{Kind: activity.RewardCoreComboSwitch, ComboID: target.ID}
			if test.name == "terms restored by 055" || test.name == "later custom in same raffle" {
				later := custom
				later.RenewalPriceMinor = 600
				rewards := []activity.Reward{custom, later, change}
				if test.name == "later custom in same raffle" {
					later.ComboID = monthly.ID
					rewards = []activity.Reward{custom, change, later}
				}
				results := settleRecoveryRaffle(t, store, user.ID, -910000-int64(index), now, rewards)
				reproduceLegacyCoreRewards(t, store, purchase.ID, results)
			} else {
				grant := playComboReward(t, store, user.ID, "custom", custom, now.Add(time.Minute))
				if test.name == "ambiguous latest custom" {
					playComboReward(t, store, user.ID, "same-time custom", custom, grant.CreatedAt)
				}
				changed := playComboReward(t, store, user.ID, "core", change, now.Add(2*time.Minute))
				if test.name == "terms restored by 054" {
					reproduceLegacyCoreRewards(t, store, purchase.ID, []activity.DrawResult{grant, changed})
				} else if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_reset_strategy=NULL WHERE id=?`, purchase.ID); err != nil {
					t.Fatal(err)
				}
			}
			switch test.name {
			case "terms restored by 054":
				reapplyRecoveryMigration(t, store, customComboRecoveryMigration)
			case "terms restored by 055":
				reapplyRecoveryMigration(t, store, orderedCustomRecoveryMigration)
			case "automatic renewal successor":
				if err := store.SetAutoRenewal(ctx, user.ID, purchase.ID, true, now); err != nil {
					t.Fatal(err)
				}
				recordAutomaticRollover(t, store, purchase.ID, purchase.ValidUntil, 1000, 0)
				var err error
				purchase, err = store.CommitAutoRenewal(ctx, purchase.ID, purchase.ValidUntil)
				if err != nil {
					t.Fatal(err)
				}
			case "explicit cadence":
				if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_reset_strategy='MONTH_ROLLING' WHERE id=?`, purchase.ID); err != nil {
					t.Fatal(err)
				}
			case "administrator edit":
				if err := store.AppendAudit(ctx, &user.ID, "entitlement.edit", "purchase", purchase.ID, "{}", now.Add(3*time.Minute)); err != nil {
					t.Fatal(err)
				}
			case "changed custom terms":
				if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET reward_renewal_price_minor=501 WHERE id=?`, purchase.ID); err != nil {
					t.Fatal(err)
				}
			}
			before, err := store.PurchaseByID(ctx, purchase.ID)
			if err != nil {
				t.Fatal(err)
			}
			snapshot := customComboRecoverySnapshot(t, store, purchase.ID)
			balance := adminWorkflowBalance(t, store, user.ID)
			for range 2 {
				reapplyRecoveryMigration(t, store, customCadenceMigration)
				after, err := store.PurchaseByID(ctx, purchase.ID)
				if err != nil || after.ResetStrategy != test.want {
					t.Fatalf("recovered cadence = (%+v, %v), want %s", after, err, test.want)
				}
				if after.ComboID != before.ComboID || after.PriceTXBMinor != before.PriceTXBMinor || after.TrafficLimitBytes != before.TrafficLimitBytes || !after.ValidUntil.Equal(before.ValidUntil) || after.AutoRenewEnabled != before.AutoRenewEnabled || adminWorkflowBalance(t, store, user.ID) != balance {
					t.Fatal("cadence repair changed other purchase terms or balance")
				}
				if !test.recover && customComboRecoverySnapshot(t, store, purchase.ID) != snapshot {
					t.Fatal("protected purchase was changed")
				}
				var audits, syncs int
				if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='activity.custom_combo_cadence_recovered' AND target_id=?`, purchase.ID).Scan(&audits); err != nil {
					t.Fatal(err)
				}
				if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='remna_sync_user' AND json_extract(payload,'$.userId')=? AND status IN ('pending','processing')`, user.ID).Scan(&syncs); err != nil {
					t.Fatal(err)
				}
				if audits != boolInt(test.recover) || syncs != 1 {
					t.Fatalf("cadence repair side effects: audits=%d syncs=%d", audits, syncs)
				}
			}
		})
	}
}
