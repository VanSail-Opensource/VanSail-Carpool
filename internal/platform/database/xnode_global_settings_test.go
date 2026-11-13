package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func globalSettingsSnapshot(t *testing.T, store *Store) string {
	t.Helper()
	var value string
	if err := store.DB().QueryRowContext(context.Background(), `SELECT jsonb_build_object(
		'settings',(SELECT COALESCE(jsonb_agg(to_jsonb(t)),'[]'::jsonb) FROM settings t WHERE key=$1),
		'version',(SELECT revision FROM xnode_compile_revision WHERE singleton=1),
		'audits',(SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY id),'[]'::jsonb) FROM audit_events t WHERE action='setting.update' AND target_id=$1))::text`,
		xnode.GlobalSettingKey).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestGlobalSettingsRecheckCurrentAdministratorAuthority(t *testing.T) {
	for _, state := range []struct {
		name, role, status string
		blocked            int
	}{
		{"demoted", "user", "active", 0}, {"purging", "admin", "purging", 0},
		{"blocked", "admin", "active", 1}, {"missing", "admin", "active", 0},
		{"empty", "admin", "active", 0},
	} {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			actor := createTestAdmin(t, store, 969011).ID
			if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role=$1,account_status=$2,bot_blocked=$3 WHERE id=$4`,
				state.role, state.status, state.blocked, actor); err != nil {
				t.Fatal(err)
			}
			if state.name == "missing" {
				actor = "missing-administrator"
			}
			if state.name == "empty" {
				actor = ""
			}
			version, _, err := store.XNodeCompileVersion(ctx, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			before := globalSettingsSnapshot(t, store)
			if err = store.PutAdminXNodeGlobalAtVersion(ctx, actor, "secret-cipher", version, time.Time{}); !errors.Is(err, xnode.ErrConflict) {
				t.Fatal("stale administrator changed fleet settings", err)
			}
			if after := globalSettingsSnapshot(t, store); after != before {
				t.Fatal("rejected authority changed settings, revision or audit")
			}
		})
	}
}

func TestGlobalSettingsFenceCompileInputsAndAuditAtomically(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	actor := createTestAdmin(t, store, 969012).ID
	version, _, err := store.XNodeCompileVersion(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	before := globalSettingsSnapshot(t, store)
	if err = store.PutAdminXNodeGlobalAtVersion(ctx, actor, "secret-cipher", version-1, time.Time{}); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("stale compile epoch committed", err)
	}
	if after := globalSettingsSnapshot(t, store); after != before {
		t.Fatal("rejected epoch changed settings or audit")
	}
	for _, statement := range []string{
		`CREATE FUNCTION reject_global_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'global audit unavailable'; END $$`,
		`CREATE TRIGGER reject_global_audit BEFORE INSERT ON audit_events FOR EACH ROW WHEN (NEW.action='setting.update') EXECUTE FUNCTION reject_global_audit()`,
	} {
		if _, err = store.DB().ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.PutAdminXNodeGlobalAtVersion(ctx, actor, "secret-cipher", version, time.Time{}); err == nil || !strings.Contains(err.Error(), "global audit unavailable") {
		t.Fatal("setting escaped failed audit", err)
	}
	if after := globalSettingsSnapshot(t, store); after != before {
		t.Fatal("failed audit retained setting or trigger revision")
	}
	if _, err = store.DB().ExecContext(ctx, `DROP TRIGGER reject_global_audit ON audit_events`); err != nil {
		t.Fatal(err)
	}
	if err = store.PutAdminXNodeGlobalAtVersion(ctx, actor, "secret-cipher", version, time.Time{}); err != nil {
		t.Fatal("repaired audit did not recover", err)
	}
	got, err := store.GetSetting(ctx, xnode.GlobalSettingKey)
	if err != nil || !got.Encrypted || got.Value != "secret-cipher" {
		t.Fatal("committed secret setting changed", got, err)
	}
	var audits int
	var detail string
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*),MIN(detail) FROM audit_events WHERE action='setting.update' AND target_id=$1 AND actor_user_id=$2`,
		xnode.GlobalSettingKey, actor).Scan(&audits, &detail); err != nil || audits != 1 || detail != `{"secret":true}` {
		t.Fatal("global audit leaked material or lost actor", audits, detail, err)
	}
	next, _, err := store.XNodeCompileVersion(ctx, time.Now())
	if err != nil || next <= version {
		t.Fatal("global settings did not invalidate compiled state", next, version, err)
	}
}
