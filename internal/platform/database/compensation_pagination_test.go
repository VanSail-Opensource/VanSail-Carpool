package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
)

func TestCompensationHistoryCombinesStatusCursorAndLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	threshold, multiplier := 1, 10_000
	config := compensation.Config{Enabled: true, ThresholdMinutes: &threshold, MultiplierBPS: &multiplier}
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	nodes := []compensation.Node{{UUID: "first", Name: "First"}, {UUID: "second", Name: "Second"}}
	if err := store.RecordCompensationObservation(ctx, config, nodes, now); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"", "observing"} {
		first, err := store.ListCompensationEvents(ctx, status, "", 1)
		if err != nil || len(first.Items) != 1 || first.NextCursor == nil {
			t.Fatalf("first page (%q): %+v, %v", status, first, err)
		}
		second, err := store.ListCompensationEvents(ctx, status, *first.NextCursor, 1)
		if err != nil || len(second.Items) != 1 || second.NextCursor != nil || second.Items[0].ID == first.Items[0].ID {
			t.Fatalf("second page (%q): %+v, %v", status, second, err)
		}
	}
	empty, err := store.ListCompensationEvents(ctx, "pending_review", "", 1)
	if err != nil || len(empty.Items) != 0 || empty.NextCursor != nil {
		t.Fatalf("unmatched status: %+v, %v", empty, err)
	}
}
