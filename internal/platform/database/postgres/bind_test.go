package postgres

import (
	"database/sql/driver"
	"strings"
	"testing"
)

func TestSQLAwareBinding(t *testing.T) {
	for _, tc := range []struct {
		name, query, want string
		args              []driver.NamedValue
	}{
		{"protected", "SELECT '?',\"?\",$$?$$,$body$?$body$,? -- ?\n/* ? /* nested ? */ */", "SELECT '?',\"?\",$$?$$,$body$?$body$,$1 -- ?\n/* ? /* nested ? */ */", []driver.NamedValue{{Ordinal: 1, Value: "value"}}},
		{"escaped-quote", "SELECT 'it''s ?' WHERE id=?", "SELECT 'it''s ?' WHERE id=$1::bigint", []driver.NamedValue{{Ordinal: 1, Value: int64(1 << 40)}}},
		{"aggregate", "SELECT MAX(id),MAX(MAX(id),1),MIN(1,2) FROM t", "SELECT MAX(id),GREATEST(MAX(id),1),LEAST(1,2) FROM t", nil},
		{"null-equality", "SELECT 1 WHERE value IS ? AND other IS NOT NULL", "SELECT 1 WHERE value IS NOT DISTINCT FROM $1 AND other IS NOT NULL", []driver.NamedValue{{Ordinal: 1, Value: nil}}},
		{"authored", "SELECT $1::text", "SELECT $1::text", []driver.NamedValue{{Ordinal: 1, Value: "literal ?"}}},
		{"concatenated-placeholder", "SELECT ?||':'||?", "SELECT $1||':'||$2", []driver.NamedValue{{Ordinal: 1, Value: "owner"}, {Ordinal: 2, Value: "node"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Bind(tc.query, tc.args)
			if err != nil || got != tc.want {
				t.Fatalf("query=%q error=%v", got, err)
			}
		})
	}
	for _, query := range []string{"SELECT '", "SELECT /* unclosed", "SELECT $$unclosed", "SELECT ?|array['x']"} {
		if _, err := Bind(query, nil); err == nil {
			t.Fatalf("accepted ambiguous/invalid SQL: %s", query)
		}
	}
	got, err := Bind("INSERT OR IGNORE INTO t(id) VALUES(?) RETURNING id", []driver.NamedValue{{Ordinal: 1, Value: "x"}})
	if err != nil || !strings.Contains(got, "ON CONFLICT DO NOTHING RETURNING id") {
		t.Fatalf("conflict conversion: %q %v", got, err)
	}
}
