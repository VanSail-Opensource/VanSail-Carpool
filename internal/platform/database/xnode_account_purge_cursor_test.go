package database

import (
	"context"
	"testing"
	"time"
)

func TestNativePurgeCursorSurvivesRestartAndOfflineFirst128(t *testing.T) {
	s, target, jobID, identity := nativePurgeFixture(t)
	ctx := context.Background()
	_, err := s.db.ExecContext(ctx, `INSERT INTO xnode_periods(id,proxy_user_id,started_at,limit_bytes,strategy,status)
 SELECT '000-offline-'||lpad(n::text,4,'0'),$1,$2,0,'NO_RESET','open' FROM generate_series(1,257) n`, identity.ProxyUserID, stamp(time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	first, allowed, err := s.AccountNativePurgePage(ctx, target, jobID, nil)
	if err != nil || !allowed || len(first) != 128 {
		t.Fatal("first bounded page", len(first), allowed, err)
	}
	seen := map[string]bool{}
	for _, period := range first {
		seen[period.ID] = true
		current, active, e := s.AdvanceAccountNativePurge(ctx, target, jobID, period)
		if e != nil || !active || current.Status != "closing" {
			t.Fatal("attempt fence", current, active, e)
		}
		// Leave all participants genuinely pending, as an offline agent would.
	}
	restarted := NewStore(s.db)
	if err = restarted.ConfigureAccountCleanupAdmins(nil); err != nil {
		t.Fatal(err)
	}
	second, allowed, err := restarted.AccountNativePurgePage(ctx, target, jobID, nil)
	if err != nil || !allowed || len(second) != 128 {
		t.Fatal("second bounded page", len(second), allowed, err)
	}
	for _, p := range second {
		if seen[p.ID] {
			t.Fatal("offline first page starved later work", p.ID)
		}
	}
	if _, _, err = restarted.AdvanceAccountNativePurge(ctx, target, jobID, second[0]); err != nil {
		t.Fatal(err)
	}
	third, _, err := s.AccountNativePurgePage(ctx, target, jobID, nil)
	if err != nil || len(third) == 0 || third[0].ID != second[1].ID {
		t.Fatal("individual attempt cursor skipped unattempted page tail", third, err)
	}
	var cursor, marker string
	if err = s.db.QueryRowContext(ctx, `SELECT payload::jsonb->>'nativePeriodCursor',payload::jsonb->>'retainedTestMarker' FROM outbox_jobs WHERE id=$1`, jobID).Scan(&cursor, &marker); err != nil || cursor != second[0].ID || marker != "preserved" {
		t.Fatal("durable cursor overwrote canonical payload", cursor, marker, err)
	}
	for _, period := range second[1:] {
		if _, _, err = s.AdvanceAccountNativePurge(ctx, target, jobID, period); err != nil {
			t.Fatal(err)
		}
	}
	later, _, err := s.AccountNativePurgePage(ctx, target, jobID, nil)
	if err != nil {
		t.Fatal(err)
	}
	hourlyReached := false
	for _, period := range later {
		if period.ProxyUserID != identity.ProxyUserID {
			hourlyReached = true
		}
	}
	if !hourlyReached {
		t.Fatal("pending legacy first256 prevented later hourly drain", later)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE outbox_jobs SET payload=jsonb_set(payload::jsonb,'{nativePeriodCursor}','"zzzz"'::jsonb)::text WHERE id=$1`, jobID); err != nil {
		t.Fatal(err)
	}
	wrapped, _, err := s.AccountNativePurgePage(ctx, target, jobID, nil)
	if err != nil || len(wrapped) == 0 || wrapped[0].ID != first[0].ID {
		t.Fatal("cursor did not wrap pending periods", wrapped, err)
	}
}
