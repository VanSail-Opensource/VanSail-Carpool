package backup

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	_ "modernc.org/sqlite"
)

func TestRunCreatesVerifiedSQLiteBackup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "source.db")
	db, err := database.Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("sql.Open(): %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, `CREATE TABLE sample(value TEXT NOT NULL); INSERT INTO sample(value) VALUES ('durable')`); err != nil {
		t.Fatalf("seed database: %v", err)
	}

	repository := &backupRepository{}
	service := NewService(db, repository, filepath.Join(t.TempDir(), "backups"), 7*24*time.Hour)
	fixedNow := time.Now().UTC()
	service.now = func() time.Time { return fixedNow }
	run, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if run.Status != "complete" || run.SizeBytes <= 0 || repository.completedErr != nil {
		t.Fatalf("backup run = %+v, completion error = %v", run, repository.completedErr)
	}

	backupDB, err := sql.Open("sqlite", "file:"+filepath.ToSlash(run.Path)+"?mode=ro")
	if err != nil {
		t.Fatalf("open published backup: %v", err)
	}
	defer func() { _ = backupDB.Close() }()
	var value string
	if err := backupDB.QueryRowContext(ctx, `SELECT value FROM sample`).Scan(&value); err != nil || value != "durable" {
		t.Fatalf("backup value = %q, error = %v", value, err)
	}
}

func TestRemoveExpiredCleansPublishedAndCrashTemporaryFiles(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	now := time.Now().UTC()
	old := now.Add(-8 * 24 * time.Hour)
	files := map[string]bool{
		"tx-carpool-old.db":      true,
		".tx-carpool-old.db.tmp": true,
		"tx-carpool-new.db":      false,
		".tx-carpool-new.db.tmp": false,
		"unrelated-old.db":       false,
	}
	for name, remove := range files {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if remove || name == "unrelated-old.db" {
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatalf("age %s: %v", name, err)
			}
		}
	}
	service := NewService(nil, nil, directory, 7*24*time.Hour)
	service.now = func() time.Time { return now }
	if err := service.RemoveExpired(); err != nil {
		t.Fatalf("RemoveExpired(): %v", err)
	}
	for name, removed := range files {
		_, err := os.Stat(filepath.Join(directory, name))
		if removed && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still exists", name)
		}
		if !removed && err != nil {
			t.Fatalf("%s was removed: %v", name, err)
		}
	}
}

func TestRemoveExpiredDeletesBackupRecordsAndMissingFiles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	directory := filepath.Join(t.TempDir(), "backups")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "records.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	oldPath := filepath.Join(directory, "tx-carpool-old.db")
	currentPath := filepath.Join(directory, "tx-carpool-current.db")
	activePath := filepath.Join(directory, "tx-carpool-active.db")
	for _, path := range []string{oldPath, currentPath, activePath} {
		if err = os.WriteFile(path, []byte("backup"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := now.Add(-8 * 24 * time.Hour).Format(time.RFC3339Nano)
	current := now.Add(-6 * 24 * time.Hour).Format(time.RFC3339Nano)
	for _, run := range []struct{ id, path, completed string }{
		{"old", oldPath, old}, {"missing", filepath.Join(directory, "tx-carpool-missing.db"), old}, {"current", currentPath, current}, {"active", activePath, old},
	} {
		if _, err = db.ExecContext(ctx, `INSERT INTO backup_runs(id,path,status,created_at,completed_at) VALUES(?,?,'complete',?,?)`, run.id, run.path, run.completed, run.completed); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO restore_jobs(id,backup_run_id,status,staged_path,rescue_path,source_sha256,created_at,updated_at) VALUES('restore-active','active','ready','stage','rescue','hash',?,?)`, old, old); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, nil, directory, 7*24*time.Hour)
	service.now = func() time.Time { return now }
	if err = service.RemoveExpired(); err != nil {
		t.Fatalf("RemoveExpired(): %v", err)
	}
	for _, id := range []string{"old", "missing"} {
		var count int
		if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM backup_runs WHERE id=?`, id).Scan(&count); err != nil || count != 0 {
			t.Fatalf("expired backup %q count = %d, error = %v", id, count, err)
		}
	}
	if _, err = os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired backup file still exists: %v", err)
	}
	if _, err = os.Stat(currentPath); err != nil {
		t.Fatalf("current backup was removed: %v", err)
	}
	if _, err = os.Stat(activePath); err != nil {
		t.Fatalf("backup with active restore was removed: %v", err)
	}
}

func TestOpenDownloadRejectsAStoredPathOutsideBackupDirectory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	directory := filepath.Join(t.TempDir(), "backups")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "tx-carpool-outside.db")
	if err := os.WriteFile(outside, []byte("not a backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "records.db")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, `CREATE TABLE backup_runs(id TEXT PRIMARY KEY,path TEXT,size_bytes INTEGER,status TEXT); INSERT INTO backup_runs VALUES('outside',?,12,'complete')`, outside); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, nil, directory, time.Hour)
	if _, err := service.OpenDownload(ctx, "outside"); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("OpenDownload outside error = %v", err)
	}
}

func TestStagedRestoreSwapsPreOpenAndRecordsCompletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	databasePath := filepath.Join(root, "tx-carpool.db")
	db, err := database.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	store := database.NewStore(db)
	if _, err := db.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at) VALUES('restore.test','snapshot',0,?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "backups")
	service := NewService(db, store, directory, 7*24*time.Hour)
	source, err := service.Run(ctx)
	if err != nil {
		t.Fatalf("Run(source): %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE settings SET value='live' WHERE key='restore.test'`); err != nil {
		t.Fatal(err)
	}
	versions, err := database.MigrationVersions()
	if err != nil {
		t.Fatal(err)
	}
	reason := "Recover the verified snapshot"
	confirmation := "RESTORE " + filepath.Base(source.Path)
	if _, err := service.StageRestore(ctx, source.ID, "", "", reason, confirmation, versions); !errors.Is(err, ErrRestoreConflict) {
		t.Fatalf("StageRestore() without key error = %v", err)
	}
	job, err := service.StageRestore(ctx, source.ID, "", "restore-swap-1", reason, confirmation, versions)
	if err != nil {
		t.Fatalf("StageRestore(): %v", err)
	}
	if job.Status != "ready" || job.RescuePath == "" {
		t.Fatalf("staged job = %+v", job)
	}
	replay, err := service.StageRestore(ctx, source.ID, "", "restore-swap-1", reason, confirmation, versions)
	if err != nil || replay.ID != job.ID || replay.Status != "ready" {
		t.Fatalf("StageRestore() replay = %+v, %v", replay, err)
	}
	if _, err := service.StageRestore(ctx, source.ID, "", "restore-swap-1", reason+" changed", confirmation, versions); !errors.Is(err, ErrRestoreConflict) {
		t.Fatalf("StageRestore() conflicting replay error = %v", err)
	}
	if _, err := os.Stat(markerPath(databasePath)); err != nil {
		t.Fatalf("restore marker: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close live database: %v", err)
	}
	result, err := ApplyPendingRestore(ctx, databasePath, versions)
	if err != nil || result == nil || result.Status != "complete" {
		t.Fatalf("ApplyPendingRestore() = %+v, %v", result, err)
	}
	restoredDB, err := database.Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("open restored database: %v", err)
	}
	t.Cleanup(func() { _ = restoredDB.Close() })
	if _, err := RecordStartupRestore(ctx, restoredDB, databasePath); err != nil {
		t.Fatalf("RecordStartupRestore(): %v", err)
	}
	var value string
	if err := restoredDB.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='restore.test'`).Scan(&value); err != nil || value != "snapshot" {
		t.Fatalf("restored value=%q, error=%v", value, err)
	}
	var status string
	if err := restoredDB.QueryRowContext(ctx, `SELECT status FROM restore_jobs WHERE id=?`, job.ID).Scan(&status); err != nil || status != "complete" {
		t.Fatalf("restore status=%q, error=%v", status, err)
	}
	restoredService := NewService(restoredDB, database.NewStore(restoredDB), directory, 7*24*time.Hour)
	replay, err = restoredService.StageRestore(ctx, source.ID, "", "restore-swap-1", reason, confirmation, versions)
	if err != nil || replay.ID != job.ID || replay.Status != "complete" {
		t.Fatalf("StageRestore() post-swap replay = %+v, %v", replay, err)
	}
	var storedKey, storedFingerprint string
	if err := restoredDB.QueryRowContext(ctx, `SELECT idempotency_key,request_fingerprint FROM restore_jobs WHERE id=?`, job.ID).
		Scan(&storedKey, &storedFingerprint); err != nil || storedKey != "restore-swap-1" || storedFingerprint != job.RequestFingerprint {
		t.Fatalf("restore replay identity = %q/%q, error=%v", storedKey, storedFingerprint, err)
	}
	if _, err := os.Stat(resultPath(databasePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recorded result sidecar still exists: %v", err)
	}
}

func TestCorruptStagedRestoreLeavesLiveDatabaseAndRecordsFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	databasePath := filepath.Join(root, "tx-carpool.db")
	db, err := database.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	store := database.NewStore(db)
	if _, err := db.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at) VALUES('restore.failure','live',0,?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, store, filepath.Join(root, "backups"), time.Hour)
	source, err := service.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	versions, _ := database.MigrationVersions()
	job, err := service.StageRestore(ctx, source.ID, "", "restore-failure-1", "Test failed restore rollback", "RESTORE "+filepath.Base(source.Path), versions)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(job.StagedPath, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyPendingRestore(ctx, databasePath, versions); err == nil {
		t.Fatal("ApplyPendingRestore() unexpectedly succeeded")
	}
	liveDB, err := database.Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("open preserved live database: %v", err)
	}
	t.Cleanup(func() { _ = liveDB.Close() })
	if _, err := RecordStartupRestore(ctx, liveDB, databasePath); err != nil {
		t.Fatalf("record failed restore: %v", err)
	}
	var value, status string
	if err := liveDB.QueryRowContext(ctx, `SELECT value FROM settings WHERE key='restore.failure'`).Scan(&value); err != nil || value != "live" {
		t.Fatalf("preserved value=%q, error=%v", value, err)
	}
	if err := liveDB.QueryRowContext(ctx, `SELECT status FROM restore_jobs WHERE id=?`, job.ID).Scan(&status); err != nil || status != "failed" {
		t.Fatalf("failed restore status=%q, error=%v", status, err)
	}
}

func TestVerifySnapshotRejectsNonPrefixMigrationMarkers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "non-prefix.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations(version TEXT PRIMARY KEY,applied_at TEXT NOT NULL);
		INSERT INTO schema_migrations(version,applied_at) VALUES('001_initial.sql','2026-08-08T00:00:00Z'),('003_emby.sql','2026-08-08T00:00:01Z')`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	err = verifySnapshot(ctx, path, []string{"001_initial.sql", "002_activity_coupons_questionnaires.sql", "003_emby.sql"})
	if err == nil || !strings.Contains(err.Error(), "ordered prefix") {
		t.Fatalf("verifySnapshot() error = %v, want ordered-prefix rejection", err)
	}
}

type backupRepository struct {
	run          model.BackupRun
	completedErr error
}

func (r *backupRepository) StartBackupRun(_ context.Context, path string, now time.Time) (model.BackupRun, error) {
	r.run = model.BackupRun{ID: "backup-1", Path: path, Status: "running", CreatedAt: now}
	return r.run, nil
}

func (r *backupRepository) CompleteBackupRun(_ context.Context, _ string, size int64, backupErr error, now time.Time) error {
	r.completedErr = backupErr
	r.run.SizeBytes = size
	r.run.CompletedAt = &now
	return nil
}
