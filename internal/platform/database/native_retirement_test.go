package database

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Build the real previous schema without editing any historical migration.
func beforeNativeRetirement(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "previous.db"))+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") || entry.Name() >= nativeRetirementMigration {
			continue
		}
		script, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name() == usdtMigration || entry.Name() == zeroMultiplierMigration {
			err = migrateReferencedTables(ctx, db, script, entry.Name())
		} else {
			_, err = db.ExecContext(ctx, string(script))
		}
		if err != nil {
			t.Fatalf("previous schema %s: %v", entry.Name(), err)
		}
		if entry.Name() != usdtMigration && entry.Name() != zeroMultiplierMigration {
			if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)`, entry.Name(), stamp(time.Now())); err != nil {
				t.Fatal(err)
			}
		}
	}
	return db
}

func TestNativeRetirementRequiresCompletedAccounting(t *testing.T) {
	for _, phase := range []string{"active", "complete"} {
		t.Run(phase, func(t *testing.T) {
			db := beforeNativeRetirement(t)
			ctx := context.Background()
			if _, err := db.ExecContext(ctx, `INSERT INTO xnode_migrations(singleton,phase,source_cipher,updated_at) VALUES(1,?,'encrypted-source',?)`, phase, stamp(time.Now())); err != nil {
				t.Fatal(err)
			}
			if err := migrate(ctx, db); err == nil || !strings.Contains(err.Error(), "finalize source accounting") {
				t.Fatal("unfinished accounting was accepted", err)
			}
			var cipher string
			if err := db.QueryRowContext(ctx, `SELECT source_cipher FROM xnode_migrations`).Scan(&cipher); err != nil || cipher != "encrypted-source" {
				t.Fatal("failed upgrade destroyed recovery state", err)
			}
		})
	}
}

func TestNativeRetirementPreservesFactsAndTranslatesPendingAccess(t *testing.T) {
	db := beforeNativeRetirement(t)
	ctx := context.Background()
	now := stamp(time.Now().UTC())
	statements := []string{
		`INSERT INTO users(id,telegram_id,remna_user_id,created_at,updated_at) VALUES('member',123,'123','` + now + `','` + now + `')`,
		`INSERT INTO balances(user_id,txb_minor,updated_at) VALUES('member',12345,'` + now + `')`,
		`INSERT INTO xnode_identities(proxy_user_id,user_id,credentials_cipher,created_at) VALUES(123,'member','unchanged-credentials','` + now + `')`,
		`INSERT INTO xnode_periods(id,proxy_user_id,started_at,limit_bytes,strategy,used_bytes,status) VALUES('period',123,'` + now + `',1000,'NO_RESET',400,'open')`,
		`INSERT INTO xnode_legacy_usage(proxy_user_id,period_id,used_bytes,lifetime_bytes,synced_at) VALUES(123,'period',300,9000,'` + now + `')`,
		`INSERT INTO xnode_legacy_history(proxy_user_id,node_id,date,raw_bytes,weighted_bytes) VALUES(123,'historical','2026-10-01',200,300)`,
		`INSERT INTO outbox_jobs(id,kind,payload,status,available_at,created_at,updated_at) VALUES('access','remna_sync_user','{"userId":"member"}','pending','` + now + `','` + now + `','` + now + `')`,
		`INSERT INTO xnode_migrations(singleton,phase,source_cipher,finalized_at,updated_at) VALUES(1,'complete','retired-secret','` + now + `','` + now + `')`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	// Earlier versions are already installed; only execute the forward cleanup.
	script, err := migrations.ReadFile("migrations/" + nativeRetirementMigration)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var balance, used, historical int64
	var credentials, kind, proxyID string
	if err = db.QueryRowContext(ctx, `SELECT b.txb_minor,p.used_bytes,h.weighted_bytes,i.credentials_cipher,j.kind,u.proxy_user_id FROM balances b JOIN users u ON u.id=b.user_id JOIN xnode_identities i ON i.user_id=u.id JOIN xnode_periods p ON p.proxy_user_id=i.proxy_user_id JOIN xnode_imported_history h ON h.proxy_user_id=i.proxy_user_id JOIN outbox_jobs j ON j.id='access'`).Scan(&balance, &used, &historical, &credentials, &kind, &proxyID); err != nil {
		t.Fatal(err)
	}
	if balance != 12345 || used != 400 || historical != 300 || credentials != "unchanged-credentials" || kind != "xnode_sync_user" || proxyID != "123" {
		t.Fatal("retirement changed exact facts", balance, used, historical, credentials, kind, proxyID)
	}
	var artifacts int
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name IN ('xnode_migrations','xnode_migration_nodes')`).Scan(&artifacts); err != nil || artifacts != 0 {
		t.Fatal("bridge artifacts retained", err)
	}
}
