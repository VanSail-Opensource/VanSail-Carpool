package database

import (
	"context"
	"database/sql"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
)

func replaceIPCacheTx(ctx context.Context, tx *sql.Tx, r iplookup.Report) error {
	if r.LiveDetails == nil {
		return nil
	} // Legacy workers keep their original storage contract.
	key := iplookup.CacheKey(r.IP)
	if _, err := tx.ExecContext(ctx, `UPDATE ip_lookup_reports SET cache_key=? WHERE id=?`, key, r.ID); err != nil {
		return err
	}
	// Creation order prevents a slower older run displacing a newer generation.
	var newer int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM ip_lookup_reports WHERE cache_key=? AND status IN ('succeeded','partial') AND rowid>(SELECT rowid FROM ip_lookup_reports WHERE id=?)`, key, r.ID).Scan(&newer); err != nil {
		return err
	}
	predicate := `cache_key=? AND status IN ('succeeded','partial') AND id<>?`
	if newer > 0 {
		predicate = `id=?`
		_, err := tx.ExecContext(ctx, `DELETE FROM ip_lookup_reports WHERE `+predicate, r.ID)
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM ip_lookup_reports WHERE `+predicate, key, r.ID)
	return err
}
