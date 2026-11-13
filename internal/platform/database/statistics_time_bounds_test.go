package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCatalogStatisticsUseExactHalfOpenTimestampRange(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
	for _, window := range []struct {
		name     string
		from, to time.Time
	}{
		{"whole seconds", base, base.Add(time.Second)},
		{"nanoseconds", base.Add(400 * time.Nanosecond), base.Add(800 * time.Nanosecond)},
	} {
		t.Run(window.name, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			included := saveTestSquad(t, store, "11111111-1111-4111-8111-111111111111", 0, true)
			addon := saveTestSquad(t, store, "22222222-2222-4222-8222-222222222222", 25, true)
			combo := saveTestCombo(t, store, "Exact interval", 100, 30, included.ID)
			for index, at := range []time.Time{window.from.Add(-time.Nanosecond), window.from, window.from.Add(time.Nanosecond), window.to, window.to.Add(time.Nanosecond)} {
				createAdminWorkflowPurchase(t, store, 93_001+int64(index), combo, at, addon.GroupID)
			}
			for _, report := range []struct {
				name                  string
				read                  func() (model.AdminStatistics, error)
				wantAddon, wantSlices int64
			}{
				{"combo", func() (model.AdminStatistics, error) {
					return store.ComboStatistics(ctx, combo.ID, window.from, window.to, "daily", time.UTC)
				}, 50, 2},
				{"included squad", func() (model.AdminStatistics, error) {
					return store.SquadStatistics(ctx, included.GroupID, window.from, window.to, "daily", time.UTC)
				}, 0, 1},
				{"addon squad", func() (model.AdminStatistics, error) {
					return store.SquadStatistics(ctx, addon.GroupID, window.from, window.to, "daily", time.UTC)
				}, 50, 1},
			} {
				t.Run(report.name, func(t *testing.T) {
					value, err := report.read()
					if err != nil || value.Count != 2 || value.InputMinor != 250 || value.AddonMinor != report.wantAddon || int64(len(value.Distribution)) != report.wantSlices {
						t.Fatalf("exact catalog report = %+v, %v", value, err)
					}
					for _, slice := range value.Distribution {
						if slice.Count != 2 {
							t.Fatalf("distribution used a different time interval: %+v", value.Distribution)
						}
					}
				})
			}
		})
	}
}

func TestActivityStatisticsUseExactHalfOpenTimestampRange(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 93_101)
	from := time.Date(2026, 11, 13, 16, 0, 0, 0, time.UTC)
	to := from.Add(time.Second)
	usageBreakdownExec(t, store, `INSERT INTO activity_games(id,name,win_chance_bps,minimum_stake_minor,maximum_stake_minor,return_multiplier_bps,created_at,updated_at)
 VALUES('time-game','Time game',5000,1,100,20000,$1,$1)`, stamp(from))
	for index, at := range []time.Time{from.Add(-time.Nanosecond), from, from.Add(time.Nanosecond), to, to.Add(time.Nanosecond)} {
		id := fmt.Sprintf("time-result-%d", index)
		usageBreakdownExec(t, store, `INSERT INTO activity_bets(id,user_id,game_id,stake_minor,won,payout_minor,balance_after_minor,configuration_snapshot,idempotency_key,created_at)
 VALUES($1,$2,'time-game',50,0,0,0,'{}',$1,$3)`, id, user.ID, stamp(at))
		usageBreakdownExec(t, store, `INSERT INTO activity_draw_results(id,user_id,draw_id,prize_id,prize_name,fee_minor,reward_kind,reward_payload,balance_after_minor,configuration_snapshot,idempotency_key,created_at)
 VALUES($1,$2,'time-draw','time-prize','Time prize',50,'none','{}',0,'{}',$1,$3)`, id, user.ID, stamp(at))
	}
	game, err := store.ActivityGameStatistics(ctx, "time-game", from, to, "daily", time.UTC)
	if err != nil || game.Count != 2 || game.InputMinor != 100 || game.Losses != 2 {
		t.Fatalf("exact game report = %+v, %v", game, err)
	}
	draw, err := store.LuckyDrawStatistics(ctx, "time-draw", from, to, "daily", time.UTC)
	if err != nil || draw.Count != 2 || draw.InputMinor != 100 || len(draw.Distribution) != 1 || draw.Distribution[0].Count != 2 {
		t.Fatalf("exact draw report = %+v, %v", draw, err)
	}
}
