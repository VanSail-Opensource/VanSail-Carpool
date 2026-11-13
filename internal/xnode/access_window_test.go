package xnode

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestContinuityUntilFollowsEveryContiguousPaidTermInAnyHistoryOrder(t *testing.T) {
	at := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	current := model.Purchase{ID: "current", UserID: "owner", Status: "active", ValidFrom: at, ValidUntil: at.Add(time.Hour)}
	first := model.Purchase{ID: "first", UserID: "owner", Status: "queued", ValidFrom: current.ValidUntil, ValidUntil: at.Add(2 * time.Hour)}
	second := model.Purchase{ID: "second", UserID: "owner", Status: "queued", ValidFrom: first.ValidUntil, ValidUntil: at.Add(3 * time.Hour)}
	third := model.Purchase{ID: "third", UserID: "owner", Status: "queued", ValidFrom: second.ValidUntil, ValidUntil: at.Add(4 * time.Hour)}
	for _, history := range [][]model.Purchase{
		{third, second, first, current}, // Authoritative repository order.
		{second, current, third, first}, // Batched compile snapshots need no order.
		{current, first, second, third},
	} {
		repo := &renewalRepository{purchases: history}
		service := NewService(repo, nil, nil, nil, nil)
		before := make([]string, len(history))
		for i, purchase := range history {
			before[i] = purchase.ID
		}
		until, err := service.continuityUntil(context.Background(), current)
		if err != nil || !until.Equal(third.ValidUntil) {
			t.Fatalf("paid chain was shortened: %s, %v", until, err)
		}
		after := make([]string, len(history))
		for i, purchase := range history {
			after[i] = purchase.ID
		}
		if !slices.Equal(before, after) {
			t.Fatal("continuity reordered the caller's cached history")
		}
	}
}

func TestContinuityUntilCannotBridgeUnpaidOrMissingTerms(t *testing.T) {
	at := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	current := model.Purchase{ID: "current", UserID: "owner", Status: "active", ValidUntil: at}
	for _, status := range []string{"cancelled", "failed", "expired"} {
		repo := &renewalRepository{purchases: []model.Purchase{
			{Status: "queued", ValidFrom: at.Add(time.Hour), ValidUntil: at.Add(2 * time.Hour)},
			{Status: status, ValidFrom: at, ValidUntil: at.Add(time.Hour)},
		}}
		service := NewService(repo, nil, nil, nil, nil)
		until, err := service.continuityUntil(context.Background(), current)
		if err != nil || !until.Equal(current.ValidUntil) {
			t.Fatalf("%s term bridged a paid gap: %s, %v", status, until, err)
		}
	}
}
