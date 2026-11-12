package postgres

import (
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestFirstSyncReconfirmationUpgradePreservesMaterialAndFinance(t *testing.T) {
	// This existing hosted fixture contains real historical accounting and
	// native credentials at schema45, inside its own disposable database.
	ctx, db := multiplierFixture(t)
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, files, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(ctx, 48); err != nil {
		t.Fatal(err)
	}
	const originalProof = "2026-11-01T00:00:00Z"
	queries := []string{
		`UPDATE xnode_identities SET first_synced_at=$1 WHERE proxy_user_id=7654`,
		`INSERT INTO users(id,telegram_id,account_generation,created_at,updated_at) VALUES('reconfirmed-owner',987654,3,$1,$1)`,
		`INSERT INTO xnode_identities(proxy_user_id,user_id,credentials_cipher,subscription_cipher,subscription_hash,first_synced_at,created_at) VALUES(987654,'reconfirmed-owner','member-material','member-token',decode(repeat('ab',32),'hex'),$1,$1)`,
		`INSERT INTO xnode_identities(proxy_user_id,service_name,credentials_cipher,subscription_cipher,subscription_hash,first_synced_at,created_at) VALUES(4503599627370500,'reconfirmation-service','service-material','service-token',decode(repeat('cd',32),'hex'),$1,$1)`,
	}
	for _, query := range queries {
		if _, err = db.ExecContext(ctx, query, originalProof); err != nil {
			t.Fatal(err)
		}
	}
	contract, err := SchemaContract()
	if err != nil {
		t.Fatal(err)
	}
	contract.Tables, err = existingUpgradeProjection(ctx, db, contract.Tables)
	if err != nil {
		t.Fatal(err)
	}
	// Only this readiness field may change. All native credential/token
	// material, periods, balances, purchases, receipts and sequences remain
	// in the exact authoritative fingerprint.
	for index, table := range contract.Tables {
		if table.Name != "xnode_identities" {
			continue
		}
		columns := make([]ColumnContract, 0, len(table.Columns))
		for _, column := range table.Columns {
			if column.Name != "first_synced_at" {
				columns = append(columns, column)
			}
		}
		contract.Tables[index].Columns = columns
	}
	before, err := inspect(ctx, db, contract.Tables)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(ctx, 49); err != nil {
		t.Fatal(err)
	}
	after, err := inspect(ctx, db, contract.Tables)
	if err != nil || before.Hash != after.Hash {
		t.Fatal("readiness repair changed authoritative material or financial evidence", err)
	}
	var remaining int
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_identities WHERE user_id IS NOT NULL AND first_synced_at IS NOT NULL`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatal("historical member readiness survived repair", remaining, err)
	}
	var serviceProof string
	if err = db.QueryRowContext(ctx, `SELECT first_synced_at FROM xnode_identities WHERE service_name='reconfirmation-service'`).Scan(&serviceProof); err != nil || serviceProof != originalProof {
		t.Fatal("member repair changed standalone service proof", serviceProof, err)
	}
	const freshProof = "2026-11-12T00:00:00Z"
	if _, err = db.ExecContext(ctx, `UPDATE xnode_identities SET first_synced_at=$1 WHERE proxy_user_id=7654`, freshProof); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal("completed readiness migration was not idempotent", err)
	}
	var persisted string
	if err = db.QueryRowContext(ctx, `SELECT first_synced_at FROM xnode_identities WHERE proxy_user_id=7654`).Scan(&persisted); err != nil || persisted != freshProof {
		t.Fatal("restart migration erased newly confirmed readiness", persisted, err)
	}
}
