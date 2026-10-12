package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
)

func TestIPLookupCurrentConfigMigrationPreservesReferencesAndRowOrder(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`PRAGMA foreign_keys=ON; CREATE TABLE ip_lookup_reports(id TEXT PRIMARY KEY,config_json TEXT NOT NULL,report_json TEXT NOT NULL);
CREATE TABLE receipts(id TEXT PRIMARY KEY,report_id TEXT REFERENCES ip_lookup_reports(id));
INSERT INTO ip_lookup_reports(rowid,id,config_json,report_json) VALUES(42,'kept','{}','{"_checkpoint":{"complete":true}}'); INSERT INTO receipts VALUES('receipt','kept');`)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := migrations.ReadFile("migrations/071_ip_lookup_current_config.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	var rowid int
	var raw string
	if err := db.QueryRow(`SELECT r.rowid,r.report_json FROM receipts c JOIN ip_lookup_reports r ON r.id=c.report_id`).Scan(&rowid, &raw); err != nil {
		t.Fatal(err)
	}
	if rowid != 42 || raw != `{"_checkpoint":{"complete":true}}` {
		t.Fatal("migration altered recovery data or ordering")
	}
	var removed int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('ip_lookup_reports') WHERE name='config_json'`).Scan(&removed); err != nil || removed != 0 {
		t.Fatal("config column retained")
	}
}

func TestIPLookupResumeAndRepairLoadCurrentSettings(t *testing.T) {
	ctx := context.Background()
	store, service, user, _, _ := ipLookupFixture(t, 2)
	op := submitIP(t, service, user, "current-settings", false)
	r, c, err := store.IPLookupRun(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	c.Retry = &iplookup.RetryPolicy{Enabled: true, MaxAttempts: 2, DelaySeconds: 321}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE settings SET value=? WHERE key=?`, string(raw), iplookup.SettingKey); err != nil {
		t.Fatal(err)
	}
	_, resumed, err := store.IPLookupRun(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, repaired, err := store.IPLookupReportByID(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Retry.DelaySeconds != 321 || repaired.Retry.DelaySeconds != 321 {
		t.Fatal("resume or repair used frozen config")
	}
}
