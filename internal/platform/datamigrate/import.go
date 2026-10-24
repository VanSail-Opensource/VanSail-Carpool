package datamigrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Import copies into an initialized fresh target in one all-or-nothing transaction.
// Migration-role ownership is required to suspend user triggers during exact copy;
// all foreign keys remain enforced and are validated before commit.
func Import(ctx context.Context, source *Source, targetURL string) (Report, error) {
	report, err := Inspect(ctx, source)
	if err != nil {
		return report, err
	}
	conn, err := pgx.Connect(ctx, targetURL)
	if err != nil {
		return report, fmt.Errorf("PostgreSQL import connection failed")
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return report, err
	}
	defer tx.Rollback(context.Background())
	var phase string
	if err = tx.QueryRow(ctx, `SELECT phase FROM vansail_migration_state WHERE singleton=1 FOR UPDATE`).Scan(&phase); err != nil {
		return report, err
	}
	if phase != "fresh" {
		return report, fmt.Errorf("target is not a fresh migration database")
	}
	seedCounts, err := bootstrapCounts()
	if err != nil {
		return report, err
	}
	var names []string
	for _, table := range source.Tables {
		var count int64
		if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM `+quote(table.Name)).Scan(&count); err != nil {
			return report, err
		}
		if count != seedCounts[table.Name] {
			return report, fmt.Errorf("refusing populated target table %s", table.Name)
		}
		names = append(names, quote(table.Name))
		if _, err = tx.Exec(ctx, `ALTER TABLE `+quote(table.Name)+` DISABLE TRIGGER USER`); err != nil {
			return report, err
		}
	}
	if _, err = tx.Exec(ctx, `TRUNCATE `+strings.Join(names, ",")); err != nil {
		return report, err
	}
	if _, err = tx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
		return report, err
	}
	for _, table := range source.Tables {
		selection, order := tableSelection(table, false)
		rows, e := source.Tx.QueryContext(ctx, `SELECT `+selection+` FROM `+quote(table.Name)+` ORDER BY `+order)
		if e != nil {
			return report, e
		}
		columns := make([]string, 0, len(table.Columns)+1)
		for _, c := range table.Columns {
			columns = append(columns, c.Name)
		}
		if hasOrdering(table.Name) {
			columns = append(columns, "record_order")
		}
		stream := &copyRows{rows: rows, width: len(columns)}
		_, e = tx.CopyFrom(ctx, pgx.Identifier{"public", table.Name}, columns, stream)
		rows.Close()
		if e != nil {
			return report, fmt.Errorf("copy table %s: %w", table.Name, e)
		}
	}
	if _, err = tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		return report, err
	}
	for _, table := range source.Tables {
		if _, err = tx.Exec(ctx, `ALTER TABLE `+quote(table.Name)+` ENABLE TRIGGER USER`); err != nil {
			return report, err
		}
	}
	if err = resetSequences(ctx, tx, report.Sequences); err != nil {
		return report, err
	}
	if _, err = tx.Exec(ctx, `UPDATE vansail_migration_state SET phase='imported',source_hash=$1,verification_hash='',verified_at='' WHERE singleton=1`, report.Hash); err != nil {
		return report, err
	}
	if err = tx.Commit(ctx); err != nil {
		return report, err
	}
	return report, nil
}
