package postgres

import (
	"context"
	"database/sql"
	"testing"
)

// Forward retirements can append audits. Every pre-upgrade audit must remain exact.
func captureUpgradeAudits(t *testing.T, ctx context.Context, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.QueryContext(ctx, `SELECT id,to_jsonb(audit_events)::text FROM audit_events`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			t.Fatal(err)
		}
		result[id] = raw
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func verifyUpgradeAudits(t *testing.T, ctx context.Context, db *sql.DB, before map[string]string) {
	t.Helper()
	after := captureUpgradeAudits(t, ctx, db)
	for id, raw := range before {
		if after[id] != raw {
			t.Fatal("historical audit changed", id)
		}
	}
	if _, exists := after["member-firewall-retirement"]; !exists {
		t.Fatal("missing member firewall retirement audit")
	}
}
