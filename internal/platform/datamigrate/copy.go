package datamigrate

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
)

type copyRows struct {
	rows  *sql.Rows
	width int
	err   error
}

func (r *copyRows) Next() bool { return r.rows.Next() }
func (r *copyRows) Err() error {
	if r.err != nil {
		return r.err
	}
	return r.rows.Err()
}
func (r *copyRows) Values() ([]any, error) {
	values := make([]any, r.width)
	dest := make([]any, r.width)
	for i := range values {
		dest[i] = &values[i]
	}
	r.err = r.rows.Scan(dest...)
	return values, r.err
}

func resetSequences(ctx context.Context, tx pgx.Tx, evidence map[string]int64) error {
	rows, err := tx.Query(ctx, `SELECT t.relname,a.attname,pg_get_serial_sequence(quote_ident(n.nspname)||'.'||quote_ident(t.relname),a.attname) FROM pg_class t JOIN pg_namespace n ON n.oid=t.relnamespace JOIN pg_attribute a ON a.attrelid=t.oid WHERE n.nspname='public' AND a.attidentity<>''`)
	if err != nil {
		return err
	}
	type sequence struct{ table, column, name string }
	var sequences []sequence
	for rows.Next() {
		var item sequence
		if err = rows.Scan(&item.table, &item.column, &item.name); err != nil {
			rows.Close()
			return err
		}
		sequences = append(sequences, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, s := range sequences {
		var maximum int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(`+quote(s.column)+`),0) FROM `+quote(s.table)).Scan(&maximum); err != nil {
			return err
		}
		maximum = max(maximum, evidence[s.table])
		if _, err = tx.Exec(ctx, `SELECT setval($1::regclass,$2,$3)`, s.name, max(1, maximum), maximum > 0); err != nil {
			return err
		}
	}
	maximum := max(int64(4503599627370496), evidence["xnode_service_proxy_id_seq"])
	_, err = tx.Exec(ctx, `SELECT setval('xnode_service_proxy_id_seq',GREATEST($1,COALESCE((SELECT MAX(proxy_user_id) FROM xnode_identities WHERE user_id IS NULL),0)),$2 OR EXISTS(SELECT 1 FROM xnode_identities WHERE user_id IS NULL AND proxy_user_id>=4503599627370496))`, maximum, evidence["xnode_service_proxy_id_seq"] >= 4503599627370496)
	return err
}
