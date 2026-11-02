package backup

import (
	"context"

	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
)

func TestRemoveExpiredCleansPublishedAndCrashTemporaryFiles(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	now := time.Now().UTC()
	old := now.Add(-8 * 24 * time.Hour)
	files := map[string]bool{
		"tx-carpool-old.dump":      true,
		".tx-carpool-old.dump.tmp": true,
		"tx-carpool-new.dump":      false,
		".tx-carpool-new.dump.tmp": false,
		"unrelated-old.dump":       false,
	}
	for name, remove := range files {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if remove || name == "unrelated-old.dump" {
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
	db, err := testdatabase.Open(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	oldPath := filepath.Join(directory, "tx-carpool-old.dump")
	currentPath := filepath.Join(directory, "tx-carpool-current.dump")
	activePath := filepath.Join(directory, "tx-carpool-active.dump")
	for _, path := range []string{oldPath, currentPath, activePath} {
		if err = os.WriteFile(path, []byte("backup"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := now.Add(-8 * 24 * time.Hour).Format(time.RFC3339Nano)
	current := now.Add(-6 * 24 * time.Hour).Format(time.RFC3339Nano)
	for _, run := range []struct{ id, path, completed string }{
		{"old", oldPath, old}, {"missing", filepath.Join(directory, "tx-carpool-missing.dump"), old}, {"current", currentPath, current}, {"active", activePath, old},
	} {
		if _, err = db.ExecContext(ctx, `INSERT INTO backup_runs(id,path,status,created_at,completed_at) VALUES($1,$2,'complete',$3,$4)`, run.id, run.path, run.completed, run.completed); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO restore_jobs(id,backup_run_id,status,staged_path,rescue_path,source_sha256,created_at,updated_at) VALUES('restore-active','active','ready','stage','rescue','hash',$1,$2)`, old, old); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, nil, directory, 7*24*time.Hour)
	service.now = func() time.Time { return now }
	if err = service.RemoveExpired(); err != nil {
		t.Fatalf("RemoveExpired(): %v", err)
	}
	for _, id := range []string{"old", "missing"} {
		var count int
		if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM backup_runs WHERE id=$1`, id).Scan(&count); err != nil || count != 0 {
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
	outside := filepath.Join(t.TempDir(), "tx-carpool-outside.dump")
	if err := os.WriteFile(outside, []byte("not a backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := testdatabase.Open(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, `INSERT INTO backup_runs(id,path,size_bytes,status,created_at) VALUES('outside',$1,12,'complete','2026-01-01T00:00:00Z')`, outside); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, nil, directory, time.Hour)
	if _, err := service.OpenDownload(ctx, "outside"); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("OpenDownload outside error = %v", err)
	}
}
