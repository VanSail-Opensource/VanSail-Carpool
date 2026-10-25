package database

import (
	"context"
	"strings"
	"testing"
)

// Hosted PostgreSQL only: duplicate reservation history exercises the aggregate
// with 2,001 terms while preserving distinct-member stock semantics.
func explainStockPlan(t *testing.T, s *Store, purchase string) {
	t.Helper()
	ctx := context.Background()
	rows, err := s.db.QueryContext(ctx, `SELECT * FROM purchases WHERE id=?`, purchase)
	if err != nil {
		t.Fatal(err)
	}
	columns, err := rows.Columns()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	names, expressions := []string{}, []string{}
	for _, column := range columns {
		quoted := `"` + strings.ReplaceAll(column, `"`, `""`) + `"`
		names = append(names, quoted)
		expression := "p." + quoted
		if column == "id" || column == "idempotency_key" {
			expression += `||'-plan-'||n::text`
		}
		expressions = append(expressions, expression)
	}
	query := `INSERT INTO purchases(` + strings.Join(names, ",") + `) SELECT ` + strings.Join(expressions, ",") + ` FROM purchases p CROSS JOIN generate_series(1,2000) n WHERE p.id=?`
	if _, err := s.db.ExecContext(ctx, query, purchase); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `ANALYZE purchases`); err != nil {
		t.Fatal(err)
	}
	counts, err := s.squadStockCounts(ctx)
	if err != nil || counts["live"] != 1 {
		t.Fatal("history double-counted a member", counts, err)
	}
	rows, err = s.db.QueryContext(ctx, "EXPLAIN (ANALYZE, BUFFERS) "+squadStockQuery)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		t.Log(line)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
