package iplookup

import (
	"context"
	"time"
)

// reclaim releases retired temporary-table pages while live generations remain indexed.
func (d *inventoryDisk) reclaim(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if len(d.generations) == 0 {
		// With no readers/builders, release heap and index pages together.
		_, err := d.db.ExecContext(ctx, `TRUNCATE pg_temp.prefixes,pg_temp.domains`)
		return err
	}
	for _, statement := range []string{`VACUUM (TRUNCATE) pg_temp.prefixes`, `VACUUM (TRUNCATE) pg_temp.domains`} {
		if _, err := d.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
