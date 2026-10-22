package database

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
)

// Rebuild only the member table, preserving all unrelated columns and named
// indexes/triggers. The original migrations remain immutable upgrade history.
func removeAssignedUsername(ctx context.Context, tx *sql.Tx) error {
	var definition string
	if err := tx.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&definition); err != nil {
		return err
	}
	field := regexp.MustCompile(`(?i)\busername\s+TEXT\s+UNIQUE\s*,`)
	if !field.MatchString(definition) {
		return errors.New("unexpected legacy username schema")
	}
	definition = field.ReplaceAllString(definition, "")
	definition = regexp.MustCompile(`(?i)^CREATE TABLE(?: IF NOT EXISTS)?\s+"?users"?`).ReplaceAllString(definition, "CREATE TABLE users_telegram_identity")
	rows, err := tx.QueryContext(ctx, `SELECT name,sql FROM sqlite_master WHERE tbl_name='users' AND type IN ('index','trigger') AND sql IS NOT NULL`)
	if err != nil {
		return err
	}
	var objects []string
	for rows.Next() {
		var name, script string
		if err = rows.Scan(&name, &script); err != nil {
			rows.Close()
			return err
		}
		objects = append(objects, script)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = tx.QueryContext(ctx, `SELECT name FROM pragma_table_info('users') WHERE name<>'username' ORDER BY cid`)
	if err != nil {
		return err
	}
	var columns []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		columns = append(columns, quoteSQLName(name))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	joined := strings.Join(columns, ",")
	for _, query := range []string{definition, "INSERT INTO users_telegram_identity(" + joined + ") SELECT " + joined + " FROM users", `DROP TABLE users`, `ALTER TABLE users_telegram_identity RENAME TO users`} {
		if _, err = tx.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	for _, script := range objects {
		if _, err = tx.ExecContext(ctx, script); err != nil {
			return err
		}
	}
	return nil
}
