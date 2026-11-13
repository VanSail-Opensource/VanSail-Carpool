package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// The archive's file data is synced before verification. Its directory entry
// must also be durable before Run can record a completed, downloadable backup.
func publishArchive(temporary, final string, syncDirectory func(string) error) error {
	if err := os.Rename(temporary, final); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(final)); err != nil {
		// The rename is already visible. Keep that archive for reconciliation,
		// but do not tell the metadata writer that publication is durable.
		return fmt.Errorf("sync PostgreSQL archive publication: %w", err)
	}
	return nil
}

func syncArchiveDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}
