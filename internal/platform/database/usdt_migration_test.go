package database

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUSDTUpgradePreservesPaymentsForeignKeysAndVerifiedBackup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upgrade.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(db)
	user := createTestUser(t, s, 88006)
	order := createTestPaymentOrder(t, s, user.ID, "ezpay", 1000, time.Now().UTC())
	if _, _, err := s.SettlePayment(ctx, "ezpay", "event", "payload", order.ID, "legacy-trade", "legacy-charge", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatal(err)
	}
	script, err := migrations.ReadFile("migrations/" + usdtMigration)
	if err != nil {
		t.Fatal(err)
	}
	legacy := strings.Split(string(script), "-- Permanent fingerprints")[0]
	legacy = strings.ReplaceAll(legacy, ",'builtin_usdt'", "")
	if _, err := conn.ExecContext(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"usdt_quotes", "usdt_transfers", "usdt_scan_cursors", "usdt_rpc_state", "usdt_rpc_usage"} {
		if _, err := conn.ExecContext(ctx, "DROP TABLE "+table); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version IN (?,?)", usdtMigration, "073_remove_usdt_rpc_quotas.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	current := NewStore(upgraded)
	retained, err := current.PaymentOrderByID(ctx, order.ID)
	if err != nil || retained.Status != "paid" || retained.ProviderTradeID == nil || *retained.ProviderTradeID != "legacy-trade" {
		t.Fatalf("legacy payment=%+v %v", retained, err)
	}
	balance, err := current.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "1000" {
		t.Fatalf("balance=%+v %v", balance, err)
	}
	rows, err := upgraded.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() {
		t.Fatal("dangling foreign key")
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	var enforcement int
	if err := upgraded.QueryRow("PRAGMA foreign_keys").Scan(&enforcement); err != nil || enforcement != 1 {
		t.Fatal("foreign key enforcement was not restored")
	}
	backups, err := filepath.Glob(path + ".pre-usdt-*.db")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v %v", backups, err)
	}
	versions, _ := MigrationVersions()
	if err := verifyMigrationBoundaryBackup(ctx, backups[0], len(versions)-2, usdtMigration); err != nil {
		t.Fatal("unverified upgrade backup", err)
	}
	createUSDTTestOrder(t, current, user.ID, "1.000")
}
