package database

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"
)

func TestRelease1CommerceMigrationBackfillsCadenceAndCoreGross(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "release1-migration.db")) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close migration database: %v", err)
		}
	})
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() >= "020_release1_commerce.sql" {
			continue
		}
		script, readErr := migrations.ReadFile("migrations/" + entry.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := db.ExecContext(ctx, string(script)); execErr != nil {
			t.Fatalf("apply %s: %v", entry.Name(), execErr)
		}
	}
	stampValue := "2026-08-01T00:00:00Z"
	if _, err := db.ExecContext(ctx, `INSERT INTO users(id,telegram_id,created_at,updated_at) VALUES('user-1',1,?,?)`, stampValue, stampValue); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO combos(id,name,price_txb_minor,validity_days,traffic_limit_bytes,
		reset_strategy,created_at,updated_at) VALUES('combo-1','Legacy',1200,30,1024,'MONTH',?,?)`, stampValue, stampValue); err != nil {
		t.Fatal(err)
	}
	legacyPurchases := []struct {
		id                        string
		charged, gross, discount  any
		addon, wantCoreGrossMinor int64
	}{
		{id: "gross-snapshot", charged: 1_200, gross: 1_500, discount: 300, addon: 300, wantCoreGrossMinor: 1_200},
		{id: "fallback-snapshot", charged: 900, gross: nil, discount: 100, addon: 200, wantCoreGrossMinor: 800},
	}
	for _, purchase := range legacyPurchases {
		_, err := db.ExecContext(ctx, `INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,valid_from,valid_until,
			status,gross_price_txb_minor,coupon_discount_txb_minor,created_at,updated_at)
			VALUES(?,'user-1','combo-1',?,'2026-08-01T00:00:00Z','2026-08-31T00:00:00Z','expired',?,?,?,?)`,
			purchase.id, purchase.charged, purchase.gross, purchase.discount, stampValue, stampValue)
		if err != nil {
			t.Fatalf("seed %s: %v", purchase.id, err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO purchase_addons(purchase_id,group_id,charged_txb_minor)
			VALUES(?,'addon',?)`, purchase.id, purchase.addon); err != nil {
			t.Fatal(err)
		}
	}
	script, err := migrations.ReadFile("migrations/020_release1_commerce.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(script)); err != nil {
		t.Fatalf("apply release migration: %v", err)
	}
	var cadence string
	if err := db.QueryRowContext(ctx, `SELECT reset_strategy FROM combos WHERE id='combo-1'`).Scan(&cadence); err != nil || cadence != "MONTH_ROLLING" {
		t.Fatalf("migrated cadence = %q, %v", cadence, err)
	}
	for _, purchase := range legacyPurchases {
		var got int64
		if err := db.QueryRowContext(ctx, `SELECT core_gross_txb_minor FROM purchases WHERE id=?`, purchase.id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != purchase.wantCoreGrossMinor {
			t.Fatalf("%s core gross = %d, want %d", purchase.id, got, purchase.wantCoreGrossMinor)
		}
	}
}
