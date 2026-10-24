package datamigrate

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

// Activate runs under the application's dedicated ownership lock, before any
// worker or bootstrap write. Imported data must still match verified evidence.
func Activate(ctx context.Context, target *sql.DB, key []byte) error {
	if err := postgres.VerifySchema(ctx, target); err != nil {
		return err
	}
	tx, err := target.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var phase, verified string
	if err = tx.QueryRowContext(ctx, `SELECT phase,verification_hash FROM vansail_migration_state WHERE singleton=1 FOR UPDATE`).Scan(&phase, &verified); err != nil {
		return err
	}
	if phase == "active" {
		return nil
	}
	if phase != "fresh" && phase != "verified" {
		return fmt.Errorf("PostgreSQL activation requires a verified import or a new installation")
	}
	contract, err := postgres.SchemaContract()
	if err != nil {
		return err
	}
	byName := map[string]postgres.TableContract{}
	for _, table := range contract.Tables {
		byName[table.Name] = table
	}
	var tables []postgres.TableContract
	for _, name := range contract.Order {
		tables = append(tables, byName[name])
	}
	if phase == "fresh" {
		seeds, e := bootstrapCounts()
		if e != nil {
			return e
		}
		for _, table := range tables {
			var count int64
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quote(table.Name)).Scan(&count); err != nil {
				return err
			}
			if count != seeds[table.Name] {
				return fmt.Errorf("unverified data in fresh PostgreSQL table %s", table.Name)
			}
		}
	} else {
		report, e := inspect(ctx, tx, tables, true)
		if e != nil {
			return e
		}
		if report.Hash != verified || report.FinancialMismatches != 0 || report.ProcessingJobs != 0 || report.ProcessingProviders != 0 {
			return fmt.Errorf("PostgreSQL activation blocked by changed data or unreconciled work")
		}
	}
	if err = VerifyEncryption(ctx, tx, key); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE vansail_migration_state SET phase='active' WHERE singleton=1`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func bootstrapCounts() (map[string]int64, error) {
	versions, err := database.MigrationVersions()
	return map[string]int64{"affiliate_config_current": 1, "affiliate_config_versions": 1, "affiliate_tiers": 1, "node_compensation_config": 1, "onboarding_content": 2, "payment_profiles": 2, "schema_migrations": int64(len(versions)), "txb_limits": 1, "xnode_compile_revision": 1, "xnode_firewall_clock": 1}, err
}
