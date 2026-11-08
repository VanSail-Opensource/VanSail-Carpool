package database

import (
	"context"
	"testing"
)

func TestNativePurgeMutationRechecksPromotionAndNewerGeneration(t *testing.T) {
	for _, state := range []struct {
		name, role, status string
		generation         int64
	}{
		{"promoted", "admin", "purging", 1}, {"newer purge", "user", "purging", 2}, {"new join", "user", "active", 2},
	} {
		t.Run(state.name, func(t *testing.T) {
			s, target, jobID, _ := nativePurgeFixture(t)
			ctx := context.Background()
			periods, active, err := s.AccountNativePurgePage(ctx, target, jobID, nil)
			if err != nil || !active || len(periods) == 0 {
				t.Fatal(periods, active, err)
			}
			if _, err = s.db.ExecContext(ctx, `UPDATE users SET role=$2,account_status=$3,account_generation=$4 WHERE id=$1`, target.UserID, state.role, state.status, state.generation); err != nil {
				t.Fatal(err)
			}
			_, active, err = s.AdvanceAccountNativePurge(ctx, target, jobID, periods[0])
			if err != nil || active {
				t.Fatal("stale purge advanced", active, err)
			}
			p, err := s.XNodePeriod(ctx, periods[0].ID)
			if err != nil || p.Status != periods[0].Status || p.ClosureGeneration != periods[0].ClosureGeneration {
				t.Fatal("stale purge fenced a native period", p, err)
			}
			var cursor string
			if err = s.db.QueryRowContext(ctx, `SELECT COALESCE(payload::jsonb->>'nativePeriodCursor','') FROM outbox_jobs WHERE id=$1`, jobID).Scan(&cursor); err != nil || cursor != "" {
				t.Fatal("stale purge changed cursor", cursor, err)
			}
		})
	}
}
