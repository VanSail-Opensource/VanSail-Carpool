package postgres

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed contract.json
var contractJSON []byte

// VerifySchema checks the inventoried columns, keys, indexes and triggers using
// PostgreSQL catalogs. Values and canonical ciphertext are never inspected here.
func VerifySchema(ctx context.Context, db *sql.DB) error {
	var contract struct {
		Tables []struct {
			Name    string `json:"name"`
			Columns []struct {
				Name       string `json:"name"`
				Type       string `json:"type"`
				NotNull    bool   `json:"notNull"`
				PrimaryKey int    `json:"primaryKey"`
			} `json:"columns"`
		} `json:"tables"`
		Indexes  []string `json:"indexes"`
		Triggers []string `json:"triggers"`
	}
	if err := json.Unmarshal(contractJSON, &contract); err != nil {
		return err
	}
	for _, table := range contract.Tables {
		for _, column := range table.Columns {
			var notNull bool
			var declared string
			var primary int
			err := db.QueryRowContext(ctx, `SELECT a.attnotnull,format_type(a.atttypid,a.atttypmod),COALESCE((SELECT array_position(c.conkey,a.attnum) FROM pg_constraint c WHERE c.conrelid=a.attrelid AND c.contype='p'),0) FROM pg_attribute a WHERE a.attrelid=to_regclass($1) AND a.attname=$2 AND a.attnum>0 AND NOT a.attisdropped`, "public."+table.Name, column.Name).Scan(&notNull, &declared, &primary)
			if err != nil {
				return fmt.Errorf("schema column %s.%s: %w", table.Name, column.Name, err)
			}
			wantType := map[string]string{"TEXT": "text", "INTEGER": "bigint", "SMALLINT": "smallint", "BLOB": "bytea", "REAL": "double precision", "DOUBLE PRECISION": "double precision"}[column.Type]
			if declared != wantType || primary != column.PrimaryKey || notNull != (column.NotNull || primary > 0) {
				return fmt.Errorf("schema contract differs at %s.%s: type=%q want=%q primary=%d want=%d notNull=%t want=%t", table.Name, column.Name, declared, wantType, primary, column.PrimaryKey, notNull, column.NotNull || primary > 0)
			}
			if column.Type == "TEXT" {
				var collation string
				if err := db.QueryRowContext(ctx, `SELECT c.collname FROM pg_attribute a JOIN pg_collation c ON c.oid=a.attcollation WHERE a.attrelid=to_regclass($1) AND a.attname=$2`, "public."+table.Name, column.Name).Scan(&collation); err != nil {
					return err
				}
				if collation != "C" {
					return fmt.Errorf("binary text ordering differs at %s.%s", table.Name, column.Name)
				}
			}
		}
	}
	for _, name := range contract.Indexes {
		var found bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_class i JOIN pg_namespace n ON n.oid=i.relnamespace WHERE n.nspname='public' AND i.relkind='i' AND i.relname=$1)`, name).Scan(&found); err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("schema index missing: %s", name)
		}
	}
	for _, name := range contract.Triggers {
		var found bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND NOT t.tgisinternal AND t.tgenabled<>'D' AND t.tgname=$1)`, name).Scan(&found); err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("schema trigger missing/disabled: %s", name)
		}
	}
	return nil
}
