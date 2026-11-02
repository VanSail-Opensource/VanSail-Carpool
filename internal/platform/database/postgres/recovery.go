package postgres

import (
	"context"
	"database/sql"
)

// InspectRecovery verifies recovery evidence in a consistent read snapshot.
// It never changes records or grants application ownership.
func InspectRecovery(ctx context.Context, target *sql.DB, key []byte) (Report, error) {
	if err := VerifyVersion(ctx, target); err != nil {
		return Report{}, err
	}
	if err := VerifySchema(ctx, target); err != nil {
		return Report{}, err
	}
	contract, err := SchemaContract()
	if err != nil {
		return Report{}, err
	}
	byName := map[string]TableContract{}
	for _, table := range contract.Tables {
		byName[table.Name] = table
	}
	var tables []TableContract
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
	return inspect(ctx, tx, tables)
}

func readSequences(ctx context.Context, q queryer) (map[string]int64, error) {
	result := make(map[string]int64)
	for table, column := range map[string]string{"xnode_identities": "proxy_user_id", "xnode_firewall_members": "ordinal"} {
		var name string
		if err := q.QueryRowContext(ctx, `SELECT pg_get_serial_sequence($1,$2)`, table, column).Scan(&name); err != nil {
			return nil, err
		}
		// pg_get_serial_sequence returns a server-quoted, schema-qualified name.
		var value int64
		var called bool
		if err := q.QueryRowContext(ctx, `SELECT last_value,is_called FROM `+name).Scan(&value, &called); err != nil {
			return nil, err
		}
		if !called {
			value = 0
		}
		result[table] = value
	}
	var value int64
	var called bool
	if err := q.QueryRowContext(ctx, `SELECT last_value,is_called FROM xnode_service_proxy_id_seq`).Scan(&value, &called); err != nil {
		return nil, err
	}
	if !called {
		value = 0
	}
	result["xnode_service_proxy_id_seq"] = value
	return result, nil
}
