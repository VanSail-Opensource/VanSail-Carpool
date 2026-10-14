package catalog

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestAutomaticRenewalUsesAwardedCustomCombo(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, baseSquad, reason string
		price, trafficGiB       int64
		missing, disabled       bool
		switchCore              bool
	}{
		{name: "lite without base squads", price: 36_000, trafficGiB: 300},
		{name: "standard with stale base squad", baseSquad: "stale", price: 54_000, trafficGiB: 500},
		{name: "max with different base squad", baseSquad: "core-squad", price: 63_000, trafficGiB: 800},
		{name: "custom then core change without included squads", price: 54_000, trafficGiB: 500, switchCore: true},
		{name: "missing awarded squad", price: 54_000, trafficGiB: 500, missing: true, reason: database.AutoRenewalReasonComboUnavailable},
		{name: "disabled awarded node", price: 54_000, trafficGiB: 500, disabled: true, reason: database.AutoRenewalReasonNoAccessibleNodes},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			db, err := database.Open(ctx, filepath.Join(t.TempDir(), "reward-renewal.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			})
			store := database.NewStore(db)
			remote := renewalTestRemote()
			service := newCatalogServiceForTest(store, remote)
			now := service.now()
			user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 51_011, FirstName: "Reward member"}, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.AdjustBalance(ctx, user.ID, 200_000, "seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			baseSquads := []string{}
			if test.baseSquad != "" {
				baseSquads = append(baseSquads, test.baseSquad)
			}
			combo, err := store.SaveCombo(ctx, database.ComboInput{Name: "Core", PriceTXBMinor: 31_900,
				ValidityDays: 30, TrafficLimitBytes: 100 << 30, ResetStrategy: "WEEK", Active: true, SquadProductIDs: baseSquads})
			if err != nil {
				t.Fatal(err)
			}
			source, err := store.CreatePurchase(ctx, database.PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "source"}, now)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.SetAutoRenewal(ctx, user.ID, source.ID, false, now); err != nil {
				t.Fatal(err)
			}
			draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{Name: "Custom combo", Kind: "instant", Enabled: true, FeeMinor: 1,
				ExpectedParticipation: 1, Prizes: []activity.PrizeInput{{Name: "Custom", ProbabilityBPS: 10_000,
					Reward: activity.Reward{Kind: activity.RewardEntitlementGrant, ComboID: combo.ID,
						SquadUUIDs: []string{"awarded"}, RenewalPriceMinor: test.price,
						TrafficLimitBytes: test.trafficGiB << 30, RolloverMinRemainingBPS: 9_999}}}}, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.PlayLuckyDraw(ctx, user.ID, draw.ID, "award", activity.CryptoRandom{}, now); err != nil {
				t.Fatal(err)
			}
			cycleDays, fees := 30, int64(1)
			if test.switchCore {
				target, err := store.SaveCombo(ctx, database.ComboInput{Name: "New core", PriceTXBMinor: 80_000,
					ValidityDays: 45, TrafficLimitBytes: 800 << 30, ResetStrategy: "MONTH_ROLLING", Active: true})
				if err != nil {
					t.Fatal(err)
				}
				change, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{Name: "Core change", Kind: "instant", Enabled: true,
					FeeMinor: 1, ExpectedParticipation: 1, Prizes: []activity.PrizeInput{{Name: "Core change", ProbabilityBPS: 10_000,
						Reward: activity.Reward{Kind: activity.RewardCoreComboSwitch, ComboID: target.ID}}}}, now)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.PlayLuckyDraw(ctx, user.ID, change.ID, "core-change", activity.CryptoRandom{}, now); err != nil {
					t.Fatal(err)
				}
				cycleDays, fees = 45, 2
			}
			if !test.missing {
				remote.squads = append(remote.squads, RemoteSquad{UUID: "awarded", Name: "Awarded"})
			}
			remote.nodes = append(remote.nodes, RemoteNode{UUID: "awarded-node", Name: "Awarded node", Disabled: test.disabled})
			remote.accessible["awarded"] = []string{"awarded-node"}
			status, err := service.SetAutomaticRenewal(ctx, user, source.ID, true)
			if test.reason != "" {
				if !errors.Is(err, ErrAutoRenewalIneligible) || status.IneligibleReason == nil || *status.IneligibleReason != test.reason {
					t.Fatalf("enable unavailable reward = (%+v, %v), want %s", status, err, test.reason)
				}
				return
			}
			if err != nil || !status.Enabled || !status.CanEnable || status.NetPrice.MinorInt64() != test.price {
				t.Fatalf("enable custom renewal = (%+v, %v)", status, err)
			}
			if !status.NextCycleEndsAt.Equal(source.ValidUntil.Add(time.Duration(cycleDays) * 24 * time.Hour)) {
				t.Fatalf("next cycle ends at %s", status.NextCycleEndsAt)
			}
			base, err := store.ComboByID(ctx, combo.ID, true)
			if err != nil || len(base.IncludedSquads) != len(baseSquads) || base.PriceTXBMinor != 31_900 {
				t.Fatalf("base combo was changed: (%+v, %v)", base, err)
			}
			if err := store.EnqueueDueEntitlementTransitions(ctx, source.ValidUntil); err != nil {
				t.Fatal(err)
			}
			if err := store.MarkRolloverProcessing(ctx, source.ID, source.ValidUntil); err != nil {
				t.Fatal(err)
			}
			if _, err := store.RecordRolloverCalculation(ctx, source.ID, model.RolloverUsageSummary{
				AllocatedBytes: test.trafficGiB << 30, UsedBytes: test.trafficGiB << 30, AlgorithmVersion: "cadence-v3",
			}, source.ValidUntil); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := service.ProcessDueAutoRenewals(ctx, source.ValidUntil); err != nil {
					t.Fatal(err)
				}
			}
			var successorID, squads, resetStrategy string
			var price, traffic, threshold int64
			if err := db.QueryRowContext(ctx, `SELECT id,entitlement_squad_uuids,reward_renewal_price_minor,
				entitlement_traffic_limit_bytes,reward_rollover_min_remaining_bps,entitlement_reset_strategy FROM purchases WHERE auto_renew_source_purchase_id=?`,
				source.ID).Scan(&successorID, &squads, &price, &traffic, &threshold, &resetStrategy); err != nil {
				t.Fatal(err)
			}
			if squads != `["awarded"]` || price != test.price || traffic != test.trafficGiB<<30 || threshold != 9_999 {
				t.Fatalf("successor reward = (%s, %d, %d, %d)", squads, price, traffic, threshold)
			}
			if resetStrategy != "WEEK" {
				t.Fatalf("successor reset strategy = %q, want the awarded weekly cadence", resetStrategy)
			}
			status, err = service.AutomaticRenewal(ctx, user, successorID)
			if err != nil || !status.CanEnable || !status.Enabled || status.NetPrice.MinorInt64() != test.price {
				t.Fatalf("successor renewal = (%+v, %v)", status, err)
			}
			if balance, err := store.Balance(ctx, user.ID); err != nil || balance.MinorInt64() != 200_000-31_900-fees-test.price {
				t.Fatalf("renewed balance = (%+v, %v)", balance, err)
			}
			var debits int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=? AND kind='automatic_renewal'`, user.ID).Scan(&debits); err != nil || debits != 1 {
				t.Fatalf("renewal debits = %d, %v", debits, err)
			}
		})
	}
}
