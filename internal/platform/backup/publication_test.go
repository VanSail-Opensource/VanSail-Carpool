package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestArchivePublicationPreservesVerifiedData(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	temporary, final := filepath.Join(directory, ".archive.tmp"), filepath.Join(directory, "archive.dump")
	if err := os.WriteFile(temporary, []byte("verified archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := publishArchive(temporary, final, syncArchiveDirectory); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(final); err != nil || string(data) != "verified archive" {
		t.Fatalf("published data = %q, %v", data, err)
	}
	if _, err := os.Stat(temporary); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary archive remained after publication: %v", err)
	}
}

func TestArchiveDirectoryFailureRetainsVisibleEvidenceWithoutSuccess(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	temporary, final := filepath.Join(directory, ".archive.tmp"), filepath.Join(directory, "archive.dump")
	if err := os.WriteFile(temporary, []byte("verified archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("directory synchronization failed")
	err := publishArchive(temporary, final, func(path string) error {
		if path != directory {
			t.Fatalf("wrong publication directory: %q", path)
		}
		if data, err := os.ReadFile(final); err != nil || string(data) != "verified archive" {
			t.Fatalf("directory sync preceded publication: %q, %v", data, err)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("uncertain durability reported success: %v", err)
	}
	if data, err := os.ReadFile(final); err != nil || string(data) != "verified archive" {
		t.Fatalf("published recovery evidence was discarded: %q, %v", data, err)
	}
}

func TestArchiveRenameFailurePreservesExistingRecoveryTarget(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	temporary, final := filepath.Join(directory, ".archive.tmp"), filepath.Join(directory, "protected")
	if err := os.WriteFile(temporary, []byte("verified archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(final, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := publishArchive(temporary, final, func(string) error {
		t.Fatal("rename failed before directory synchronization")
		return nil
	}); err == nil {
		t.Fatal("rename over a directory unexpectedly succeeded")
	}
	if data, err := os.ReadFile(temporary); err != nil || string(data) != "verified archive" {
		t.Fatalf("failed rename discarded the archive: %q, %v", data, err)
	}
	if info, err := os.Stat(final); err != nil || !info.IsDir() {
		t.Fatalf("failed rename changed protected target: %v", err)
	}
}
