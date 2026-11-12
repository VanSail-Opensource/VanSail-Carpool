package database

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestManualBackupAdmissionCommitsActorBeforeExport(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	actor := createTestUser(t, s, 95101)
	now := time.Now().UTC()
	run, err := s.StartAuditedBackupRun(ctx, "/managed/tx-carpool.dump", actor.ID, now)
	if err != nil || run.ID == "" || run.Status != "running" {
		t.Fatal(run, err)
	}
	var recordedActor, status, detail string
	err = s.db.QueryRowContext(ctx, `SELECT a.actor_user_id,b.status,a.detail
		FROM audit_events a JOIN backup_runs b ON b.id=a.target_id
		WHERE a.action='backup.create' AND a.target_type='backup' AND b.id=$1`, run.ID).
		Scan(&recordedActor, &status, &detail)
	if err != nil || recordedActor != actor.ID || status != "running" || detail != `{"phase":"requested"}` {
		t.Fatal("backup admission is not durably attributed", recordedActor, status, detail, err)
	}
	if strings.Contains(detail, run.Path) {
		t.Fatal("audit exposed a host path")
	}
}

func TestManualBackupAuditFailurePreventsAdmission(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	actor := createTestUser(t, s, 95102)
	restore := rejectPrivilegedAuditWrites(t, s)
	run, err := s.StartAuditedBackupRun(ctx, "/managed/tx-carpool.dump", actor.ID, time.Now())
	if err == nil || run.ID != "" {
		t.Fatal("backup admitted without its audit", run, err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM backup_runs").Scan(&count); err != nil || count != 0 {
		t.Fatal("backup metadata escaped rollback", count, err)
	}
	restore()
	if _, err := s.StartAuditedBackupRun(ctx, "/managed/tx-carpool.dump", "", time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatal("anonymous manual backup accepted", err)
	}
}

func TestAdminSettingAndAuditCommitTogether(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	actor := createTestUser(t, s, 95103)
	now := time.Now().UTC()
	const key = "provider_api_secret"
	const first = "encrypted-before"
	const second = "encrypted-after"
	if err := s.PutAdminSetting(ctx, key, first, true, actor.ID, now); err != nil {
		t.Fatal(err)
	}
	var auditActor, detail string
	if err := s.db.QueryRowContext(ctx, "SELECT actor_user_id,detail FROM audit_events WHERE action='setting.update' AND target_id=$1", key).
		Scan(&auditActor, &detail); err != nil {
		t.Fatal(err)
	}
	var audit map[string]bool
	if err := json.Unmarshal([]byte(detail), &audit); err != nil || !audit["secret"] || auditActor != actor.ID || strings.Contains(detail, first) {
		t.Fatal("setting audit leaked a value or lost its actor", detail, auditActor, err)
	}
	restore := rejectPrivilegedAuditWrites(t, s)
	if err := s.PutAdminSetting(ctx, key, second, true, actor.ID, now.Add(time.Second)); err == nil {
		t.Fatal("setting changed without a durable audit")
	}
	setting, err := s.GetSetting(ctx, key)
	if err != nil || setting.Value != first || !setting.Encrypted || !setting.UpdatedAt.Equal(now) {
		t.Fatal("setting mutation escaped audit rollback", setting, err)
	}
	restore()
	if err := s.PutAdminSetting(ctx, key, second, true, actor.ID, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	setting, err = s.GetSetting(ctx, key)
	if err != nil || setting.Value != second {
		t.Fatal("retry did not commit setting", setting, err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE action='setting.update' AND target_id=$1", key).Scan(&count); err != nil || count != 2 {
		t.Fatal("setting attempts and audits diverged", count, err)
	}
}

func rejectPrivilegedAuditWrites(t *testing.T, s *Store) func() {
	t.Helper()
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, `CREATE FUNCTION reject_privileged_audit() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER reject_privileged_audit BEFORE INSERT ON audit_events
		FOR EACH ROW EXECUTE FUNCTION reject_privileged_audit()`); err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		if _, err := s.db.ExecContext(ctx, "DROP TRIGGER reject_privileged_audit ON audit_events"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, "DROP FUNCTION reject_privileged_audit()"); err != nil {
			t.Fatal(err)
		}
	}
}
