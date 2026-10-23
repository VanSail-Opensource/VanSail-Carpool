package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

// Queued sealed targets use operation-specific contexts. X-Node commands and
// route tests bind the actor and retry key; Emby and legacy connection drops
// bind the owner and request fingerprint. Rekey only those known contexts before
// owner references change so unrelated ciphertext is never probed.
func rekeyQueuedSealedTargets(ctx context.Context, tx *sql.Tx, vault *secret.Vault, key identityRekey) error {
	rows, err := tx.QueryContext(ctx, `SELECT j.rowid,j.payload,o.kind,COALESCE(o.owner_user_id,''),o.actor_user_id,o.idempotency_key,o.request_fingerprint FROM outbox_jobs j JOIN provider_operations o ON o.id=json_extract(j.payload,'$.operationId') WHERE j.kind='provider_operation' AND j.status IN ('pending','processing','failed') AND (o.actor_user_id=? OR o.owner_user_id=?) AND o.kind IN ('xnode_command','xnode_route_speedtest','emby_preferences','emby_password','connection_drop')`, key.oldUser, key.oldUser)
	if err != nil {
		return err
	}
	type command struct {
		id                                         int64
		payload, kind, owner, actor, retry, digest string
	}
	var commands []command
	for rows.Next() {
		var item command
		if err = rows.Scan(&item.id, &item.payload, &item.kind, &item.owner, &item.actor, &item.retry, &item.digest); err != nil {
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
		before, ok := queuedSealedTargetContext(item.kind, item.actor, item.owner, item.retry, item.digest, key.oldUser)
		if !ok {
			continue
		}
		actor, owner := item.actor, item.owner
		if actor == key.oldUser {
			actor = key.user
		}
		if owner == key.oldUser {
			owner = key.user
		}
		after, ok := queuedSealedTargetContext(item.kind, actor, owner, item.retry, item.digest, key.user)
		if !ok {
			return errors.New("queued sealed target owner is invalid")
		}
		var payload map[string]json.RawMessage
		if err = json.Unmarshal([]byte(item.payload), &payload); err != nil {
			return err
		}
		sealedTarget, ok := payload["sealedTarget"]
		if !ok {
			continue
		}
		var cipher string
		if err = json.Unmarshal(sealedTarget, &cipher); err != nil {
			return err
		}
		plain, err := vault.Decrypt(before, cipher)
		if err != nil {
			return err
		}
		sealed, err := vault.Encrypt(after, plain)
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

func queuedSealedTargetContext(kind, actor, owner, key, fingerprint, user string) (string, bool) {
	switch kind {
	case "xnode_command":
		return "xnode.command." + actor + "." + key, actor == user
	case "xnode_route_speedtest":
		return "xnode.route-speedtest." + actor + "." + key, actor == user
	case "emby_preferences", "emby_password":
		return "emby-operation:" + owner + ":" + kind + ":" + fingerprint, owner == user
	case "connection_drop":
		return "connection-drop:" + owner + ":" + fingerprint, owner == user
	default:
		return "", false
	}
}
