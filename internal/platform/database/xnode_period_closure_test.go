package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestReopenedClosureRejectsOldParticipantEvidenceAtIdenticalTime(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "stream"}[stream], func(t *testing.T) {
			store, _, period := xnodeTrafficFixture(t)
			ctx := context.Background()
			if err := store.TrackXNodePeriodNode(ctx, period.ID, "node"); err != nil {
				t.Fatal(err)
			}
			at := time.Now().UTC()
			old, err := store.BeginXNodePeriodClosure(ctx, period, at)
			if err != nil || old.ClosureGeneration != 1 {
				t.Fatal(old, err)
			}
			barrier := wire.TrafficBarrier{AgentID: "agent", AccountingPeriod: period.ID, Quiesced: true}
			if stream {
				barrier.StreamID = "11111111111111111111111111111111"
				if err = store.RegisterXNodeTrafficStream(ctx, "node", "agent", barrier.StreamID); err != nil {
					t.Fatal(err)
				}
			}
			if err = store.SealXNodeClosingPeriodNode(ctx, old, "node", barrier); err != nil {
				t.Fatal(err)
			}
			if err = store.ReopenXNodePeriod(ctx, period.ID); err != nil {
				t.Fatal(err)
			}
			// A fresh store models recovery: the closure epoch is durable, not a cache.
			recovered := NewStore(store.DB())
			reopened, err := recovered.XNodePeriod(ctx, period.ID)
			if err != nil || reopened.ClosureGeneration != old.ClosureGeneration {
				t.Fatal(reopened, err)
			}
			current, err := recovered.BeginXNodePeriodClosure(ctx, reopened, at)
			if err != nil || current.ClosureGeneration != 2 || !current.EndedAt.Equal(*old.EndedAt) {
				t.Fatal(current, err)
			}
			if _, err = recovered.BeginXNodePeriodClosure(ctx, old, at); !errors.Is(err, xnode.ErrConflict) {
				t.Fatal("old operation adopted a newer closure", err)
			}
			if err = recovered.SealXNodeClosingPeriodNode(ctx, old, "node", barrier); !errors.Is(err, xnode.ErrConflict) {
				t.Fatal("old barrier sealed a newer generation", err)
			}
			if err = recovered.FinalizeXNodePeriod(ctx, old); !errors.Is(err, xnode.ErrConflict) {
				t.Fatal("old closure finalized a newer generation", err)
			}
			participants, err := recovered.XNodePeriodNodes(ctx, period.ID)
			if err != nil || len(participants) != 1 || participants[0].Flushed {
				t.Fatal("reopen retained stale barrier evidence", participants, err)
			}
			if err = recovered.FinalizeXNodePeriod(ctx, current); !errors.Is(err, xnode.ErrConflict) {
				t.Fatal("unflushed participant was finalized", err)
			}
			if err = recovered.SealXNodeClosingPeriodNode(ctx, current, "node", barrier); err != nil {
				t.Fatal(err)
			}
			if err = recovered.FinalizeXNodePeriod(ctx, current); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPeriodFinalizationAcceptsEquivalentRetainedTimestamp(t *testing.T) {
	s, _, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	closing, err := s.BeginXNodePeriodClosure(ctx, period, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE xnode_periods SET ended_at=$1 WHERE id=$2`, stamp(at.Add(time.Nanosecond)), period.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.FinalizeXNodePeriod(ctx, closing); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("changed exact closure instant accepted stale proof", err)
	}
	retained := at.Format("2006-01-02T15:04:05.000000000Z")
	if _, err = s.db.ExecContext(ctx, `UPDATE xnode_periods SET ended_at=$1 WHERE id=$2`, retained, period.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.FinalizeXNodePeriod(ctx, closing); err != nil {
		t.Fatal("equivalent timestamp spelling blocked retained closure", err)
	}
	var actual, status string
	if err = s.db.QueryRowContext(ctx, `SELECT ended_at,status FROM xnode_periods WHERE id=$1`, period.ID).Scan(&actual, &status); err != nil || actual != retained || status != "closed" {
		t.Fatal("finalization rewrote retained timestamp evidence", actual, status, err)
	}
}
