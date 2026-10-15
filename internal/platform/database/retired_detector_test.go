package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestRetireDetectorUpgradePreservesHistoryAndManualRestrictions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := preDetectorRetirementStore(t)
	user := createTestUser(t, store, 88091)
	now := time.Now().UTC()
	if _, err := store.AdjustBalance(ctx, user.ID, 1234, "retirement-test", "preserved", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAdminTemporaryBan(ctx, AdminTemporaryBanInput{ActorUserID: user.ID, UserID: user.ID, Reason: "manual restriction", DurationMinutes: 60, IdempotencyKey: "manual-ban", RequestFingerprint: "manual-ban-fingerprint"}, now); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"confirmed", "ambiguous"} {
		var completed any
		if id == "confirmed" {
			completed = stamp(now)
		}
		if _, err := store.db.ExecContext(ctx, `INSERT INTO abuse_records(id,user_id,incident_bucket_at,measured_qps,qps_limit,selected_action,created_at,punishment_completed_at) VALUES(?,?,?,10,1,'temporary_ban',?,?)`, id, user.ID, id, stamp(now), completed); err != nil {
			t.Fatal(err)
		}
		if _, err := store.db.ExecContext(ctx, `INSERT INTO abuse_temp_bans(record_id,user_id,expires_at,created_at) VALUES(?,?,?,?)`, id, user.ID, stamp(now.Add(time.Hour)), stamp(now)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO abuse_node_credentials(node_uuid,node_name,key_digest,sealed_key,rotated_at,created_at,updated_at) VALUES('node','node','hash','secret',?,?,?)`, stamp(now), stamp(now), stamp(now)); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"abuse_punishment", "abuse_restore", "abuse_notification"} {
		if _, err := store.db.ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,aggregate_id,payload,status,available_at,created_at,updated_at) VALUES(?,?,?,'{}','pending',?,?,?)`, kind, kind, user.ID, stamp(now), stamp(now), stamp(now)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version='078_retire_abuse_detector.sql'`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, store.db); err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]int{
		`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('abuse_node_credentials','abuse_policy','abuse_domain_rules','abuse_whitelist','abuse_punishment_rules','abuse_log_fingerprints','abuse_qps_samples','abuse_detector_state','abuse_pending_log_events','abuse_qps_rollups','abuse_ip_ban_scans')`: 0,
		`SELECT COUNT(*) FROM abuse_records`: 2,
		`SELECT COUNT(*) FROM outbox_jobs WHERE kind='proxy_restriction_release' AND status='pending'`:                    1,
		`SELECT COUNT(*) FROM outbox_jobs WHERE kind LIKE 'abuse_%' AND status='failed' AND last_error='FEATURE_RETIRED'`: 3,
		`SELECT COUNT(*) FROM audit_events WHERE action='migration.detector_restriction_review'`:                          1,
	} {
		var got int
		if err := store.db.QueryRowContext(ctx, query).Scan(&got); err != nil || got != want {
			t.Fatalf("%s: got=%d want=%d err=%v", query, got, want, err)
		}
	}
	if _, pending, err := store.RetiredDetectorReleaseUser(ctx, user.ID); err != nil || !pending {
		t.Fatalf("pending=%v %v", pending, err)
	}
	for range 2 {
		if err := store.CompleteRetiredDetectorRelease(ctx, user.ID, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, pending, err := store.RetiredDetectorReleaseUser(ctx, user.ID); err != nil || pending {
		t.Fatalf("pending=%v %v", pending, err)
	}
	var ambiguousRestored sql.NullString
	if err := store.db.QueryRowContext(ctx, `SELECT restored_at FROM abuse_temp_bans WHERE record_id='ambiguous'`).Scan(&ambiguousRestored); err != nil || ambiguousRestored.Valid {
		t.Fatalf("ambiguous mutation changed: %v %v", ambiguousRestored, err)
	}
	manual, err := store.ActiveAdminTemporaryBan(ctx, user.ID)
	if err != nil || manual == nil || manual.Reason != "manual restriction" {
		t.Fatalf("manual ban=%+v %v", manual, err)
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "1234" {
		t.Fatalf("balance=%+v %v", balance, err)
	}
	var releases int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='migration.detector_restriction_released'`).Scan(&releases); err != nil || releases != 1 {
		t.Fatalf("release audits=%d %v", releases, err)
	}
	rows, err := store.db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("dangling foreign key after retirement")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

// Apply the real embedded migrations through 077 before exercising the upgrade.
func preDetectorRetirementStore(t *testing.T) *Store {
	t.Helper()
	testDatabaseOpenGate <- struct{}{}
	defer func() { <-testDatabaseOpenGate }()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "pre-retirement.db"))+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), testDatabaseOpenTimeout)
	defer cancel()
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations(version TEXT PRIMARY KEY,applied_at TEXT NOT NULL); INSERT INTO schema_migrations VALUES('078_retire_abuse_detector.sql','fixture')`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return NewStore(db)
}
