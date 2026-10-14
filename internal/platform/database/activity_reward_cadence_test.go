package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func saveCadenceCombo(t *testing.T, store *Store, name, cadence string) model.Combo {
	t.Helper()
	combo, err := store.SaveCombo(context.Background(), ComboInput{Name: name, PriceTXBMinor: 100,
		ValidityDays: 30, TrafficLimitBytes: 100 << 30, ResetStrategy: cadence, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	return combo
}

func TestCustomAwardCadenceSurvivesCoreChange(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	for index, test := range []struct {
		name, original, target, override, want string
		legacy                                 bool
	}{
		{"daily to weekly", "DAY", "WEEK", "", "DAY", false},
		{"weekly to monthly", "WEEK", "MONTH_ROLLING", "", "WEEK", false},
		{"monthly to daily", "MONTH_ROLLING", "DAY", "", "MONTH_ROLLING", false},
		{"legacy inherited weekly cadence", "WEEK", "DAY", "", "WEEK", true},
		{"explicit cadence survives core change", "WEEK", "MONTH_ROLLING", "DAY", "DAY", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			core := saveCadenceCombo(t, store, "Award core", test.original)
			target := saveCadenceCombo(t, store, "Target core", test.target)
			user, purchase := createAdminWorkflowPurchase(t, store, int64(32100+index), core, now)
			custom := activity.Reward{Kind: activity.RewardEntitlementGrant, ComboID: core.ID,
				SquadUUIDs: []string{"awarded"}, RenewalPriceMinor: 500, TrafficLimitBytes: 500 << 30, RolloverMinRemainingBPS: 9999}
			playComboReward(t, store, user.ID, "custom", custom, now)
			if test.legacy || test.override != "" {
				var override any
				if test.override != "" {
					override = test.override
				}
				if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_reset_strategy=? WHERE id=?`, override, purchase.ID); err != nil {
					t.Fatal(err)
				}
			}
			playComboReward(t, store, user.ID, "core", activity.Reward{Kind: activity.RewardCoreComboSwitch, ComboID: target.ID}, now.Add(time.Minute))
			current, err := store.PurchaseByID(ctx, purchase.ID)
			if err != nil || current.ComboID != target.ID || current.ResetStrategy != test.want {
				t.Fatalf("custom cadence after core change = (%+v, %v), want %s", current, err, test.want)
			}
			plan, err := store.AutoRenewalPlan(ctx, user.ID, purchase.ID, now)
			if err != nil || plan.resetStrategyOverride == nil || *plan.resetStrategyOverride != test.want {
				t.Fatalf("renewal cadence = (%+v, %v)", plan, err)
			}
			// A later full custom award adopts its selected core's cadence and
			// does not inherit a previous custom/admin cadence override.
			custom.ComboID = target.ID
			playComboReward(t, store, user.ID, "later custom", custom, now.Add(2*time.Minute))
			current, err = store.PurchaseByID(ctx, purchase.ID)
			if err != nil || current.ResetStrategy != test.target {
				t.Fatalf("later custom cadence = (%+v, %v), want %s", current, err, test.target)
			}
		})
	}
}

func TestOrdinaryCoreChangeUsesTargetResetCadence(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	core := saveCadenceCombo(t, store, "Original", "DAY")
	target := saveCadenceCombo(t, store, "Target", "WEEK")
	user, purchase := createAdminWorkflowPurchase(t, store, 32110, core, now)
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_reset_strategy='MONTH_ROLLING' WHERE id=?`, purchase.ID); err != nil {
		t.Fatal(err)
	}
	playComboReward(t, store, user.ID, "core", activity.Reward{Kind: activity.RewardCoreComboSwitch, ComboID: target.ID}, now)
	current, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil || current.ResetStrategy != "WEEK" {
		t.Fatalf("ordinary core cadence = (%+v, %v)", current, err)
	}
}
