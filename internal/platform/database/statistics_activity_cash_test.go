package database

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestLuckyDrawStatisticsUseSettledCashForRangesAndMultipliers(t *testing.T) {
	t.Parallel()
	for _, item := range []struct {
		name        string
		kind        activity.RewardKind
		value, cash int64
	}{
		{"ranged reward", activity.RewardTXBDelta, 250, 250},
		{"ranged penalty", activity.RewardTXBDelta, -250, -250},
		{"balance gain", activity.RewardBalanceMultiplier, 15000, 450},
		{"balance reduction", activity.RewardBalanceMultiplier, 5000, -450},
	} {
		t.Run(item.name, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			now := time.Date(2026, 11, 13, 20, 0, 0, 0, time.UTC)
			user := createTestUser(t, store, 89118)
			if _, err := store.AdjustBalance(ctx, user.ID, 1000, "cash-statistics-seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{
				Name: item.name, Kind: "instant", Enabled: true, FeeMinor: 100, ExpectedParticipation: 1,
				Prizes: []activity.PrizeInput{{Name: item.name, ProbabilityBPS: 10000, Reward: activity.Reward{
					Kind: item.kind, Range: &activity.ValueRange{Min: item.value, Max: item.value, Distribution: "uniform"},
				}}},
			}, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.PlayLuckyDraw(ctx, user.ID, draw.ID, "settled-statistics", fixedActivityRandom{}, now); err != nil {
				t.Fatal(err)
			}
			statistics, err := store.LuckyDrawStatistics(ctx, draw.ID, now.Add(-time.Minute), now.Add(time.Minute), "daily", time.UTC)
			if err != nil {
				t.Fatal(err)
			}
			if statistics.Count != 1 || statistics.InputMinor != 100 || statistics.OutputMinor != item.cash || statistics.NetMinor != 100-item.cash {
				t.Fatalf("settled draw statistics = %+v", statistics)
			}
			if balance := adminWorkflowBalance(t, store, user.ID); balance != 1000-statistics.NetMinor {
				t.Fatalf("statistics disagree with settled wallet: %d vs %+v", balance, statistics)
			}
			if len(statistics.Series) != 1 || statistics.Series[0].OutputMinor != item.cash || statistics.Series[0].NetMinor != statistics.NetMinor {
				t.Fatalf("settled draw series = %+v", statistics.Series)
			}
		})
	}
}

func TestStatisticMoneyNeverWrapsAcrossSignedBoundaries(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 13, 20, 0, 0, 0, time.UTC)
	for _, item := range []struct {
		name   string
		events []statisticEvent
	}{
		{"total input", []statisticEvent{{input: math.MaxInt64}, {input: 1}}},
		{"total output", []statisticEvent{{output: math.MinInt64}, {output: -1}}},
		{"total discount", []statisticEvent{{discount: math.MaxInt64}, {discount: 1}}},
		{"total addons", []statisticEvent{{addon: math.MaxInt64}, {addon: 1}}},
		{"net income", []statisticEvent{{input: math.MaxInt64, output: -1}}},
	} {
		t.Run(item.name, func(t *testing.T) {
			if _, err := aggregateStatistics("resource", now, now.Add(time.Hour), "daily", time.UTC, item.events, nil); !errors.Is(err, ErrConflict) {
				t.Fatalf("overflowing statistics = %v", err)
			}
		})
	}
	valid, err := aggregateStatistics("resource", now, now.Add(time.Hour), "daily", time.UTC,
		[]statisticEvent{{input: math.MaxInt64, output: math.MaxInt64}}, nil)
	if err != nil || valid.InputMinor != math.MaxInt64 || valid.NetMinor != 0 {
		t.Fatalf("representable signed boundary = %+v, %v", valid, err)
	}
}

func TestAverageStatisticAmountRoundsWithoutOverflow(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	for _, item := range []struct {
		name, query string
		want        int64
	}{
		{"half maximum", `SELECT 9223372036854775807::bigint,2::bigint`, math.MaxInt64/2 + 1},
		{"large total", `SELECT 18446744073709551614::numeric,2::bigint`, math.MaxInt64},
		{"large count", `SELECT 18446744073709551614::numeric,18446744073709551614::numeric`, 1},
		{"empty", `SELECT 0::bigint,0::bigint`, 0},
	} {
		t.Run(item.name, func(t *testing.T) {
			average, err := aggregateAverageMinor(context.Background(), store.DB(), item.query)
			if err != nil || average != item.want {
				t.Fatalf("average statistic amount = %d, %v; want %d", average, err, item.want)
			}
		})
	}
	for _, query := range []string{
		`SELECT 18446744073709551615::numeric,2::bigint`,
		`SELECT -1::bigint,1::bigint`,
		`SELECT 1::bigint,-1::bigint`,
	} {
		if _, err := aggregateAverageMinor(context.Background(), store.DB(), query); !errors.Is(err, ErrConflict) {
			t.Fatalf("invalid or unrepresentable average = %v", err)
		}
	}
}
