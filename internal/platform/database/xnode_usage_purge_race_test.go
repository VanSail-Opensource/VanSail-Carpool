package database

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestUsageInsertRacingCompletedPurgeRetriesWithoutRepopulatingHistory(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprint("batch=", batch), func(t *testing.T) {
			s, identity, period, p, at := usageHistoryFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			prepareUsagePurge(t, s, identity.UserID, at)
			const key int64 = 81766345
			gate, err := s.DB().Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer gate.Close()
			if _, err = gate.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, key); err != nil {
				t.Fatal(err)
			}
			locked := true
			started := false
			finished := false
			done := make(chan error, 1)
			defer func() {
				if locked {
					release, stop := context.WithTimeout(context.Background(), time.Second)
					defer stop()
					if _, err := gate.ExecContext(release, `SELECT pg_advisory_unlock($1)`, key); err != nil {
						t.Error(err)
					}
				}
				if started && !finished {
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						t.Error("traffic worker did not stop")
					}
				}
			}()
			// Pause the actual writer after eligibility read but before its INSERT.
			// Purge's DELETE sees the same owner predicate; serializable SSI forces
			// the late writer to retry against the committed purged/tombstoned state.
			ddl := fmt.Sprintf(`CREATE FUNCTION usage_purge_gate() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(%d); RETURN NEW; END $$`, key)
			if _, err = s.DB().ExecContext(ctx, ddl); err != nil {
				t.Fatal(err)
			}
			if _, err = s.DB().ExecContext(ctx, `CREATE TRIGGER usage_purge_gate BEFORE INSERT ON xnode_usage_minutes FOR EACH ROW EXECUTE FUNCTION usage_purge_gate()`); err != nil {
				t.Fatal(err)
			}
			report := usageHistoryReport(identity, period, p, at, 1)
			report.Samples = nil
			started = true
			go func() { done <- commitUsageHistoryFixture(t, s, report, batch) }()
			blocked := false
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if err = s.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted AND objid=$1)`, key).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if !blocked {
				t.Fatal("writer did not reach the eligibility/write barrier")
			}
			if err = s.CompleteAccountPurge(ctx, identity.UserID, 1, time.Now().UTC()); err != nil {
				t.Fatal("purge failed while presentation writer waited", err)
			}
			if _, err = gate.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, key); err != nil {
				t.Fatal(err)
			}
			locked = false
			select {
			case err = <-done:
				finished = true
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if err != nil {
				t.Fatal("journal did not retry its serializable conflict", err)
			}
			var rows int
			if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM xnode_usage_minutes WHERE owner_id=$1`, identity.UserID).Scan(&rows); err != nil || rows != 0 {
				t.Fatal("concurrent insert escaped scrub", rows, err)
			}
			if sequence, _, err := s.XNodeStreamSequence(ctx, "agent", usageHistoryStream); err != nil || sequence != 1 {
				t.Fatal("suppressed history lost journal receipt", sequence, err)
			}
			if err = commitUsageHistoryFixture(t, s, report, batch); err != nil {
				t.Fatal("exact journal replay changed", err)
			}
		})
	}
}
