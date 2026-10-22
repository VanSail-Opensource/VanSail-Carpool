package database

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

// Queued administrator commands bind their encrypted target to actor+retry key.
// Rekey that context before actor references change, preserving the exact target.
func rekeyQueuedSealedTargets(ctx context.Context, tx *sql.Tx, vault *secret.Vault, key identityRekey) error {
	rows, err := tx.QueryContext(ctx, `SELECT j.rowid,j.payload,o.kind,o.idempotency_key FROM outbox_jobs j JOIN provider_operations o ON o.id=json_extract(j.payload,'$.operationId') WHERE j.kind='provider_operation' AND j.status IN ('pending','processing','failed') AND o.actor_user_id=? AND o.kind IN ('xnode_command','xnode_migration')`, key.oldUser)
	if err != nil {
		return err
	}
	type command struct {
		id                   int64
		payload, kind, retry string
	}
	var commands []command
	for rows.Next() {
		var item command
		if err = rows.Scan(&item.id, &item.payload, &item.kind, &item.retry); err != nil {
			rows.Close()
			return err
		}
		commands = append(commands, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range commands {
		var payload map[string]json.RawMessage
		if err = json.Unmarshal([]byte(item.payload), &payload); err != nil {
			return err
		}
		var cipher string
		if err = json.Unmarshal(payload["sealedTarget"], &cipher); err != nil {
			return err
		}
		prefix := "xnode.command."
		if item.kind == "xnode_migration" {
			prefix = "xnode.migration.operation."
		}
		plain, err := vault.Decrypt(prefix+key.oldUser+"."+item.retry, cipher)
		if err != nil {
			return err
		}
		sealed, err := vault.Encrypt(prefix+key.user+"."+item.retry, plain)
		if err != nil {
			return err
		}
		payload["sealedTarget"], err = json.Marshal(sealed)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET payload=? WHERE rowid=?`, string(encoded), item.id); err != nil {
			return err
		}
	}
	return nil
}
