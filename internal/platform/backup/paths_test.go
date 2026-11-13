package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupPathsRejectSymlinkedParentsAndUnmanagedNames(t *testing.T) {
	t.Parallel()
	directory, outside := t.TempDir(), t.TempDir()
	name := "tx-carpool-outside.dump"
	if err := os.WriteFile(filepath.Join(outside, name), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "linked")); err != nil {
		t.Fatal(err)
	}
	unmanaged := filepath.Join(directory, "unrelated.dump")
	if err := os.WriteFile(unmanaged, []byte("operator data"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService(nil, nil, directory, time.Hour)
	for _, path := range []string{filepath.Join(directory, "linked", name), unmanaged} {
		if file, _, err := service.openStoredFile(path); err == nil {
			file.Close()
			t.Fatalf("download accepted unmanaged path %q", path)
		}
		for _, allowMissing := range []bool{false, true} {
			if _, err := service.resolveBackupFile(path, allowMissing); err == nil {
				t.Fatalf("deletion accepted unmanaged path %q", path)
			}
		}
	}
	for _, extension := range []string{".dump", ".db"} {
		path := filepath.Join(directory, "tx-carpool-retained"+extension)
		if err := os.WriteFile(path, []byte("backup"), 0o600); err != nil {
			t.Fatal(err)
		}
		file, _, err := service.openStoredFile(path)
		if err != nil {
			t.Fatal("managed export rejected", err)
		}
		file.Close()
		if _, err := service.resolveBackupFile(path, false); err != nil {
			t.Fatal("managed export deletion rejected", err)
		}
	}
}
