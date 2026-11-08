package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestUsageBreakdownPublicReadKeepsOneSnapshotAcrossConcurrentCharges(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	a := usageBreakdownPAYGAccount(t, s, u, "snapshot-node", at)
	usageBreakdownPAYGCharge(t, s, a, "before", 0, 1, 10, at)
	usageBreakdownRaw(t, s, u, "combo", "before", "snapshot-node", 1, 0, at)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	gate, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback()
	// The first owner read establishes the real public method's snapshot. Hold
	// its next relation read, then commit both later traffic and a later charge.
	if _, err = gate.ExecContext(ctx, `LOCK TABLE purchases IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		value model.UsageBreakdown
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		v, e := s.XNodeUsageBreakdown(ctx, u.ID, at.Add(-72*time.Hour), at.Add(time.Minute))
		done <- outcome{v, e}
	}()
	blocked := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err = s.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND strpos(query,'SELECT id,charged_txb_minor,purchased_term_seconds FROM purchases')>0)`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("projection did not establish its snapshot before relation wait")
	}
	usageBreakdownPAYGCharge(t, s, a, "after", 1, 1, 20, at)
	usageBreakdownRaw(t, s, u, "combo", "after", "snapshot-node", 2, 0, at)
	if err = gate.Commit(); err != nil {
		t.Fatal(err)
	}
	var first outcome
	select {
	case first = <-done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if first.err != nil || first.value.Cost.ActualMinor != "10" || len(first.value.Nodes) != 1 || first.value.Nodes[0].UploadBytes != "1" {
		t.Fatal("projection mixed independent snapshots", first.value, first.err)
	}
	second, err := s.XNodeUsageBreakdown(ctx, u.ID, at.Add(-72*time.Hour), at.Add(time.Minute))
	if err != nil || second.Cost.ActualMinor != "30" || len(second.Nodes) != 1 || second.Nodes[0].UploadBytes != "3" {
		t.Fatal("subsequent read did not see committed facts", second, err)
	}
	assertUsageBreakdownConservation(t, first.value)
	assertUsageBreakdownConservation(t, second)
}
