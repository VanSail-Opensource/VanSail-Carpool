package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestProductStatisticsPreserveRetainedSquadsAndActivityRewards(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	included := saveTestSquad(t, store, "11111111-1111-4111-8111-111111111111", 0, true)
	replacement := saveTestSquad(t, store, "22222222-2222-4222-8222-222222222222", 0, true)
	addon := saveTestSquad(t, store, "33333333-3333-4333-8333-333333333333", 25, true)
	reward := saveTestSquad(t, store, "44444444-4444-4444-8444-444444444444", 0, true)
	payg := saveTestSquad(t, store, "55555555-5555-4555-8555-555555555555", 0, true)
	if _, err := store.SaveSquadProduct(ctx, SquadProductInput{GroupID: payg.GroupID, UpstreamPresent: true,
		BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
		t.Fatal(err)
	}
	// Filtering the PAYG group produces a durable base snapshot. Ordinary NULL
	// snapshots retain their existing live-catalog semantics.
	combo := saveTestCombo(t, store, "Retained squad statistics", 100, 30, included.ID, payg.ID)
	member, purchase := createAdminWorkflowPurchase(t, store, 72_004, combo, now, addon.GroupID)
	var snapshot string
	if err := store.DB().QueryRowContext(ctx, `SELECT combo_squad_uuids FROM purchases WHERE id=$1`, purchase.ID).Scan(&snapshot); err != nil || snapshot != `["`+included.GroupID+`"]` {
		t.Fatalf("filtered purchase snapshot = %q, %v", snapshot, err)
	}
	draw, err := store.SaveLuckyDraw(ctx, activity.LuckyDrawInput{
		Name: "Statistics reward", Kind: "instant", Enabled: true, FeeMinor: 100,
		ExpectedParticipation: 1, Prizes: []activity.PrizeInput{{
			Name: "Extra squad", ProbabilityBPS: 10_000,
			Reward: activity.Reward{Kind: activity.RewardSquadAccess, SquadUUIDs: []string{reward.GroupID}},
		}},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PlayLuckyDraw(ctx, member.ID, draw.ID, "statistics-reward", fixedActivityRandom{}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveCombo(ctx, ComboInput{
		ID: combo.ID, Name: combo.Name, PriceTXBMinor: combo.PriceTXBMinor,
		ValidityDays: combo.ValidityDays, TrafficLimitBytes: combo.TrafficLimitBytes,
		ResetStrategy: combo.ResetStrategy, SquadProductIDs: []string{replacement.GroupID}, Active: true,
	}); err != nil {
		t.Fatal(err)
	}

	statistics, err := store.ProductDatabaseStatistics(ctx, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if statistics.AverageOptionalSquads != 2 {
		t.Fatalf("optional squads = %v, want retained addon and reward", statistics.AverageOptionalSquads)
	}
	want := map[string]bool{included.GroupID: true, addon.GroupID: true, reward.GroupID: true}
	if len(statistics.ComboBySquad) != len(want) {
		t.Fatalf("retained squad distribution = %+v", statistics.ComboBySquad)
	}
	for _, distribution := range statistics.ComboBySquad {
		if !want[distribution.ID] || len(distribution.Segments) != 1 ||
			distribution.Segments[0].ID != combo.ID || distribution.Segments[0].Value != 100 {
			t.Fatalf("unexpected distribution after catalog edit: %+v", distribution)
		}
	}
}
