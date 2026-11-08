package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestClosingPeriodRejectsNewAndRepeatedAdmissionWithoutReopening(t *testing.T) {
	store, _, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	if err := store.TrackXNodePeriodNode(ctx, period.ID, "node"); err != nil {
		t.Fatal(err)
	}
	period.Status = "closing"
	at := time.Now().UTC()
	period.EndedAt = &at
	if err := store.SaveXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
	for _, node := range []string{"node", "late-node"} {
		if err := store.TrackXNodePeriodNode(ctx, period.ID, node); !errors.Is(err, xnode.ErrConflict) {
			t.Fatal("closing period granted admission", node, err)
		}
	}
	participants, err := store.XNodePeriodNodes(ctx, period.ID)
	if err != nil || len(participants) != 1 || participants[0].NodeID != "node" {
		t.Fatal(participants, err)
	}
	period.Status = "closed"
	if err = store.SaveXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
	period.Status = "closing"
	if err = store.SaveXNodePeriod(ctx, period); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("stale closure reopened final accounting", err)
	}
}

func TestClosingPeriodScanRotatesPastBlockedOldestBatch(t *testing.T) {
	store, _, template := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	for i := range 5 {
		period := template
		period.ID, period.Status, period.EndedAt = fmt.Sprintf("closing-%d", i), "closing", &at
		if err := store.SaveXNodePeriod(ctx, period); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for range 3 {
		periods, err := store.XNodeClosingPeriods(ctx, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, period := range periods {
			seen[period.ID] = true
		}
	}
	if len(seen) != 5 {
		t.Fatal("blocked oldest periods starved later settlements", seen)
	}
}
