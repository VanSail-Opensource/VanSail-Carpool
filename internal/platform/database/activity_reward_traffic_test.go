package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestDrawTrafficKeepsTemporaryAndRenewalAllowancesSeparate(t *testing.T) {
	for _, test := range []struct {
		name                            string
		first, second                   int64
		firstRecurring, secondRecurring bool
		wantCurrent, wantRenewal        int64
	}{
		{"temporary gain then recurring gain", 10, 20, false, true, 130, 120},
		{"temporary loss then recurring gain", -10, 20, false, true, 110, 120},
		{"recurring gain then temporary loss", 20, -10, true, false, 110, 120},
		{"temporary gain then recurring loss", 20, -10, false, true, 110, 90},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
			user := createTestUser(t, store, 31831)
			if _, err := store.AdjustBalance(ctx, user.ID, 10000, "seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			combo := saveTestCombo(t, store, "Traffic term", 100, 30)
			if _, err := store.DB().ExecContext(ctx, `UPDATE combos SET traffic_limit_bytes=? WHERE id=?`, 100*(1<<30), combo.ID); err != nil {
				t.Fatal(err)
			}
			purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "purchase"}, now)
			if err != nil {
				t.Fatal(err)
			}
			for index, grant := range []struct {
				value     int64
				recurring bool
			}{{test.first, test.firstRecurring}, {test.second, test.secondRecurring}} {
				draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{
					Name: "Traffic", Kind: "instant", Enabled: true, FeeMinor: 100, ExpectedParticipation: 1,
					Prizes: []activity.PrizeInput{{Name: "Traffic", ProbabilityBPS: 10000, Reward: activity.Reward{
						Kind: activity.RewardTrafficGrant, IncludeInRenewal: grant.recurring,
						Range: &activity.ValueRange{Min: grant.value, Max: grant.value, Distribution: "uniform"},
					}}},
				}, now)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = store.PlayLuckyDraw(ctx, user.ID, draw.ID, fmt.Sprintf("grant-%d", index), fixedActivityRandom{}, now); err != nil {
					t.Fatal(err)
				}
			}
			var current int64
			if err = store.DB().QueryRowContext(ctx, `SELECT entitlement_traffic_limit_bytes FROM purchases WHERE id=?`, purchase.ID).Scan(&current); err != nil {
				t.Fatal(err)
			}
			plan, err := store.autoRenewalPlan(ctx, user.ID, purchase.ID, nil, now)
			if err != nil || plan.trafficLimitOverride == nil {
				t.Fatalf("renewal plan = %+v, %v", plan, err)
			}
			if current != test.wantCurrent*(1<<30) || *plan.trafficLimitOverride != test.wantRenewal*(1<<30) {
				t.Fatalf("traffic current=%d renewal=%d", current/(1<<30), *plan.trafficLimitOverride/(1<<30))
			}
		})
	}
}

func TestDrawTrafficRejectsLossBeyondRenewalAllowance(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	user := createTestUser(t, store, 31832)
	if _, err := store.AdjustBalance(ctx, user.ID, 10000, "seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, store, "Small renewal", 100, 30)
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "purchase"}, now)
	if err != nil {
		t.Fatal(err)
	}
	// The current term has a temporary 10 GiB allowance; renewal remains 100 MiB.
	if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_traffic_limit_bytes=?,reward_traffic_renewal=0 WHERE id=?`, 10*(1<<30), purchase.ID); err != nil {
		t.Fatal(err)
	}
	reward := activity.Reward{Kind: activity.RewardTrafficGrant, IncludeInRenewal: true,
		Range: &activity.ValueRange{Min: -1, Max: -1, Distribution: "uniform"}}
	draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{Name: "Renewal loss", Kind: "instant", Enabled: true, FeeMinor: 100,
		ExpectedParticipation: 1, Prizes: []activity.PrizeInput{{Name: "Loss", ProbabilityBPS: 10000, Reward: reward}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	rng := &countingActivityRandom{}
	if _, err = store.PlayLuckyDraw(ctx, user.ID, draw.ID, "loss", rng, now); !errors.Is(err, ErrConflict) || rng.calls != 0 {
		t.Fatalf("unsafe recurring loss: %v, rolls %d", err, rng.calls)
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err = raffleTrafficCoverageTx(ctx, tx, user.ID, draw, 2, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("raffle renewal coverage: %v", err)
	}
	if err = applyDrawTrafficTx(ctx, tx, purchase.ID, 10*(1<<30), -1, true, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("settlement renewal guard: %v", err)
	}
}
