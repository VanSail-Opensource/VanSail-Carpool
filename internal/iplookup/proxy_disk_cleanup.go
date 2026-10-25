package iplookup

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// Only these disposable, application-owned files are removed on boot/shutdown.
// Never recurse through the cache directory or delete operator files.
func removeInventoryFiles(dir string) error {
	for _, name := range []string{"inventory.db", "inventory.db-wal", "inventory.db-shm", "inventory.db-journal"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// reclaim runs with mu held. A single 256-page vacuum per minute cannot catch
// up with large retired sources. Drain free pages in bounded batches under the
// caller's maintenance deadline; live generations and their readers stay intact.
func (d *inventoryDisk) reclaim(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var free int64
		if err := d.db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&free); err != nil {
			return err
		}
		if free == 0 {
			return nil
		}
		rows, err := d.db.QueryContext(ctx, "PRAGMA incremental_vacuum(256)")
		if err != nil {
			return err
		}
		// SQLite can yield a row per reclaimed page. Consume the entire statement.
		for rows.Next() {
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		var after int64
		if err := d.db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&after); err != nil {
			return err
		}
		if after >= free {
			return nil
		}
	}
}
