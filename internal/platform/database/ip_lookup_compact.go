package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

// compactIPLookupReportsTx removes provider snapshots while retaining resume markers.
// Bounded batches keep startup migration memory independent of report history size.
func compactIPLookupReportsTx(ctx context.Context, tx *sql.Tx) error {
	type snapshot struct {
		rowID int64
		id    string
		raw   []byte
	}
	var cursor int64
	for {
		rows, err := tx.QueryContext(ctx, `SELECT rowid,id,report_json FROM ip_lookup_reports WHERE rowid>? ORDER BY rowid LIMIT 100`, cursor)
		if err != nil {
			return err
		}
		batch := make([]snapshot, 0, 100)
		for rows.Next() {
			var item snapshot
			if err := rows.Scan(&item.rowID, &item.id, &item.raw); err != nil {
				_ = rows.Close()
				return err
			}
			batch = append(batch, item)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		for _, item := range batch {
			report, err := iplookup.DecodeReport(item.raw)
			if err != nil {
				return fmt.Errorf("decode IP report %s: %w", item.id, err)
			}
			compact, err := iplookup.EncodeReport(report)
			if err != nil {
				return fmt.Errorf("compact IP report %s: %w", item.id, err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE ip_lookup_reports SET report_json=? WHERE id=?`, string(compact), item.id); err != nil {
				return err
			}
			cursor = item.rowID
		}
	}
}
