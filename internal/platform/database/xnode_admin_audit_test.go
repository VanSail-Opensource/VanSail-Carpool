package database

import (
	"context"
	"strings"
	"testing"
)

func TestNativeAdminWritesAndAuditCommitTogether(t *testing.T) {
	t.Parallel()
	ctx, f := context.Background(), newXNodeAdminFixture(t)
	for _, statement := range []string{
		`CREATE FUNCTION reject_native_admin_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'native audit unavailable'; END $$`,
		`CREATE TRIGGER reject_native_admin_audit BEFORE INSERT ON audit_events FOR EACH ROW WHEN (NEW.action LIKE 'xnode.%') EXECUTE FUNCTION reject_native_admin_audit()`,
	} {
		if _, err := f.store.DB().ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	before := xnodeAdminSnapshot(t, f.store)
	attempts := f.attempts()
	for _, attempt := range attempts {
		if err := attempt.write(f.actor); err == nil || !strings.Contains(err.Error(), "native audit unavailable") {
			t.Fatalf("%s did not reach atomic audit: %v", attempt.name, err)
		}
		if after := xnodeAdminSnapshot(t, f.store); after != before {
			t.Fatalf("%s retained mutation after failed audit", attempt.name)
		}
	}
	if _, err := f.store.DB().ExecContext(ctx, `DROP TRIGGER reject_native_admin_audit ON audit_events`); err != nil {
		t.Fatal(err)
	}
	for _, attempt := range attempts {
		if err := attempt.write(f.actor); err != nil {
			t.Fatalf("%s did not recover after audit repair: %v", attempt.name, err)
		}
	}
	var count int
	var details string
	if err := f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(string_agg(detail,' '),'') FROM audit_events WHERE actor_user_id=$1 AND action LIKE 'xnode.%'`, f.actor).Scan(&count, &details); err != nil || count != len(attempts) {
		t.Fatalf("administrator audits=%d want=%d: %v", count, len(attempts), err)
	}
	if strings.Contains(details, "secret-") || strings.Contains(details, "cipher") || strings.Contains(details, "subscription_hash") {
		t.Fatal("native audit exposed credential/configuration material")
	}
	if err := f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND action='xnode.group_deleted' AND target_type='xnode_group' AND target_id='delete-group'`, f.actor).Scan(&count); err != nil || count != 1 {
		t.Fatal("group retention audit did not retain exact actor attribution", count, err)
	}
}
