package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestGlobalSettingsExpireWhileWaitingForAuditLock(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	store := newTestStore(t)
	actor := createTestAdmin(t, store, 969013).ID
	for _, statement := range []string{
		`CREATE FUNCTION wait_global_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(969013); RETURN NEW; END $$`,
		`CREATE TRIGGER wait_global_audit BEFORE INSERT ON audit_events FOR EACH ROW WHEN (NEW.action='setting.update') EXECUTE FUNCTION wait_global_audit()`,
	} {
		if _, err := store.DB().ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	blocker, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback()
	if _, err = blocker.ExecContext(ctx, `SELECT pg_advisory_xact_lock(969013)`); err != nil {
		t.Fatal(err)
	}
	version, _, err := store.XNodeCompileVersion(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	before := globalSettingsSnapshot(t, store)
	var until time.Time
	if err = store.DB().QueryRowContext(ctx, `SELECT clock_timestamp() + INTERVAL '1 second'`).Scan(&until); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- store.PutAdminXNodeGlobalAtVersion(ctx, actor, "secret-cipher", version, until) }()
	defer func() { cancel(); _ = blocker.Rollback(); <-done }()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		var waiting bool
		if err = store.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
			AND wait_event_type='Lock' AND wait_event='advisory' AND query LIKE '%audit_events%')`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-done:
			done <- err
			t.Fatal("write never waited for the audit", err)
		case <-tick.C:
		case <-ctx.Done():
			t.Fatal("audit lock was never reached", ctx.Err())
		}
	}
	timer := time.NewTimer(max(time.Until(until)+20*time.Millisecond, 0))
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err = blocker.Rollback(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		done <- err
		if !errors.Is(err, xnode.ErrConflict) {
			t.Fatal("expired validation committed after waiting for audit", err)
		}
	case <-ctx.Done():
		t.Fatal("global writer did not finish", ctx.Err())
	}
	if after := globalSettingsSnapshot(t, store); after != before {
		t.Fatal("expired write retained settings, version or audit")
	}
}
