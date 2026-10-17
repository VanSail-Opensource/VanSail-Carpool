package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"
)

// migrateReferencedTables uses SQLite's documented table rebuild procedure,
// retaining named indexes/triggers and returning only an FK-enforced connection.
func migrateReferencedTables(ctx context.Context, db *sql.DB, script []byte, version string) (result error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil && result == nil {
			result = err
		}
	}()
	var legacy int
	if err = conn.QueryRowContext(ctx, "PRAGMA legacy_alter_table").Scan(&legacy); err != nil {
		return err
	}
	defer func() {
		restore, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, restoreErr := conn.ExecContext(restore, fmt.Sprintf("PRAGMA legacy_alter_table=%d; PRAGMA foreign_keys=ON", legacy))
		var enabled int
		if restoreErr == nil {
			restoreErr = conn.QueryRowContext(restore, "PRAGMA foreign_keys").Scan(&enabled)
		}
		if restoreErr != nil || enabled != 1 {
			result = errors.Join(result, fmt.Errorf("restore migration connection enforcement: %v", restoreErr))
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	if _, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF; PRAGMA legacy_alter_table=ON"); err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	type schemaObject struct{ name, script string }
	var objects []schemaObject
	rows, err := tx.QueryContext(ctx, "SELECT name,sql FROM sqlite_master WHERE type IN ('index','trigger') AND sql IS NOT NULL ORDER BY type,name")
	if err != nil {
		return err
	}
	for rows.Next() {
		var item schemaObject
		if err = rows.Scan(&item.name, &item.script); err != nil {
			break
		}
		objects = append(objects, item)
	}
	if err == nil {
		err = rows.Err()
	}
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, string(script)); err != nil {
		return err
	}
	for _, item := range objects {
		var present int
		if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE name=?", item.name).Scan(&present); err != nil {
			return err
		}
		if present == 0 {
			if _, err = tx.ExecContext(ctx, item.script); err != nil {
				return fmt.Errorf("restore migration schema object %s: %w", item.name, err)
			}
		}
	}
	rows, err = tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	invalid := rows.Next()
	rowErr := rows.Err()
	closeErr := rows.Close()
	if invalid || rowErr != nil || closeErr != nil {
		return fmt.Errorf("migration %s foreign key validation failed", version)
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)", version, stamp(time.Now().UTC())); err != nil {
		return err
	}
	return tx.Commit()
}
