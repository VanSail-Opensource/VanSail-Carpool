package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func replaceIPCacheTx(ctx context.Context, tx *sql.Tx, r iplookup.Report) error {
	if r.LiveDetails == nil {
		return nil
	} // Legacy workers keep their original storage contract.
	key := iplookup.CacheKey(r.IP)
	if _, err := tx.ExecContext(ctx, `UPDATE ip_lookup_reports SET cache_key=$1 WHERE id=$2`, key, r.ID); err != nil {
		return err
	}
	// Creation order prevents a slower older run displacing a newer generation.
	var newer int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM ip_lookup_reports WHERE cache_key=$1 AND status IN ('succeeded','partial') AND record_order>(SELECT record_order FROM ip_lookup_reports WHERE id=$2)`, key, r.ID).Scan(&newer); err != nil {
		return err
	}
	predicate := `cache_key=$1 AND status IN ('succeeded','partial') AND id<>$2`
	if newer > 0 {
		predicate = `id=$1`
		_, err := tx.ExecContext(ctx, `DELETE FROM ip_lookup_reports WHERE `+predicate, r.ID)
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM ip_lookup_reports WHERE `+predicate, key, r.ID)
	return err
}
