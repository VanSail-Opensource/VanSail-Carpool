package datamigrate

import (
	"context"
	"database/sql"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

// InspectPostgres verifies recovery evidence in a consistent read snapshot.
// It does not change cutover state or permit an imported database to start.
func InspectPostgres(ctx context.Context, target *sql.DB, key []byte) (Report, error) {
	if err := postgres.VerifyVersion(ctx, target); err != nil {
		return Report{}, err
	}
	if err := postgres.VerifySchema(ctx, target); err != nil {
		return Report{}, err
	}
	contract, err := postgres.SchemaContract()
	if err != nil {
		return Report{}, err
	}
	byName := map[string]postgres.TableContract{}
	for _, table := range contract.Tables {
		byName[table.Name] = table
	}
	var tables []postgres.TableContract
	for _, name := range contract.Order {
		tables = append(tables, byName[name])
	}
	tx, err := target.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return Report{}, err
	}
	defer tx.Rollback()
	if err = VerifyEncryption(ctx, tx, key); err != nil {
		return Report{}, err
	}
	return inspect(ctx, tx, tables, true)
}
