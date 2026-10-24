package datamigrate

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

// Verify compares a frozen snapshot with a consistent PostgreSQL read transaction,
// including all authoritative rows and ordering evidence, then records the gate.
func Verify(ctx context.Context, source *Source, target *sql.DB, masterKey []byte) (Report, error) {
	before, err := Inspect(ctx, source)
	if err != nil {
		return before, err
	}
	if err = VerifyEncryption(ctx, source.Tx, masterKey); err != nil {
		return before, err
	}
	if err = postgres.VerifyVersion(ctx, target); err != nil {
		return before, err
	}
	if err = postgres.VerifySchema(ctx, target); err != nil {
		return before, err
	}
	tx, err := target.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return before, err
	}
	defer tx.Rollback()
	var phase, sourceHash string
	if err = tx.QueryRowContext(ctx, `SELECT phase,source_hash FROM vansail_migration_state WHERE singleton=1 FOR UPDATE`).Scan(&phase, &sourceHash); err != nil {
		return before, err
	}
	if phase != "imported" && phase != "verified" {
		return before, fmt.Errorf("verification requires an inactive imported target")
	}
	after, err := inspect(ctx, tx, source.Tables, true)
	if err != nil {
		return after, err
	}
	if sourceHash != before.Hash || before.Hash != after.Hash {
		return after, fmt.Errorf("authoritative data hashes differ")
	}
	if after.FinancialMismatches != 0 {
		return after, fmt.Errorf("financial reconciliation anomalies require source-side review")
	}
	if err = VerifyEncryption(ctx, tx, masterKey); err != nil {
		return after, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE vansail_migration_state SET phase='verified',verification_hash=?,verified_at=? WHERE singleton=1`, after.Hash, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return after, err
	}
	return after, tx.Commit()
}

// CutoverCheck is read-only. Uncertain in-flight provider or job work blocks
// activation; operators must reconcile with the compatible source application.
func CutoverCheck(ctx context.Context, target *sql.DB) (Report, error) {
	contract, err := postgres.SchemaContract()
	if err != nil {
		return Report{}, err
	}
	tx, err := target.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return Report{}, err
	}
	defer tx.Rollback()
	var phase, verified string
	if err = tx.QueryRowContext(ctx, `SELECT phase,verification_hash FROM vansail_migration_state WHERE singleton=1`).Scan(&phase, &verified); err != nil {
		return Report{}, err
	}
	if phase != "verified" {
		return Report{}, fmt.Errorf("target has not passed inactive data verification")
	}
	byName := map[string]postgres.TableContract{}
	for _, t := range contract.Tables {
		byName[t.Name] = t
	}
	var tables []postgres.TableContract
	for _, name := range contract.Order {
		tables = append(tables, byName[name])
	}
	report, err := inspect(ctx, tx, tables, true)
	if err != nil {
		return report, err
	}
	if report.Hash != verified || report.FinancialMismatches > 0 || report.ProcessingJobs > 0 || report.ProcessingProviders > 0 {
		return report, fmt.Errorf("cutover blocked: changed data, financial anomaly or unreconciled in-flight work")
	}
	return report, nil
}
