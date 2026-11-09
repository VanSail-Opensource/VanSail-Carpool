package postgres

import (
	"context"
	"database/sql"
	"testing"
)

func captureTrafficBoundaryEvidence(t *testing.T, ctx context.Context, db *sql.DB) map[string][]string {
	t.Helper()
	evidence := map[string][]string{}
	queries := map[string]string{
		"xnode_report_cursors": `SELECT to_jsonb(r)::text FROM xnode_report_cursors r ORDER BY agent_id`,
		"xnode_node_rates":     `SELECT to_jsonb(r)::text FROM xnode_node_rates r LEFT JOIN xnode_report_cursors c USING(agent_id) WHERE (c.agent_id IS NULL AND NOT EXISTS(SELECT 1 FROM xnode_nodes n WHERE n.agent_id=r.agent_id)) OR r.from_sequence<=c.last_sequence ORDER BY r.agent_id,r.from_sequence`,
		"xnode_stream_rates":   `SELECT to_jsonb(r)::text FROM xnode_stream_rates r JOIN xnode_traffic_streams c USING(agent_id,stream_id) WHERE r.from_sequence<=c.last_sequence ORDER BY r.agent_id,r.stream_id,r.from_sequence`,
	}
	for table, query := range queries {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			continue
		}
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var row string
			if err = rows.Scan(&row); err != nil {
				t.Fatal(err)
			}
			evidence[table] = append(evidence[table], row)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	return evidence
}

func verifyTrafficBoundaryEvidence(t *testing.T, ctx context.Context, db *sql.DB, before map[string][]string) {
	t.Helper()
	for table, rows := range before {
		for _, row := range rows {
			var count int
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM `+quote(table)+` r WHERE to_jsonb(r)=$1::jsonb`, row).Scan(&count); err != nil || count != 1 {
				t.Fatal("committed historical boundary evidence changed", table, count, err)
			}
		}
	}
}
