package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
)

func rekeyQueuedIdentityPayloads(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT old_user_id,user_id,old_proxy_id,proxy_id FROM member_identity_aliases`)
	if err != nil {
		return err
	}
	users, proxies := map[string]string{}, map[string]string{}
	for rows.Next() {
		var key identityRekey
		if err = rows.Scan(&key.oldUser, &key.user, &key.oldProxy, &key.proxy); err != nil {
			rows.Close()
			return err
		}
		if key.user != "" {
			users[key.oldUser] = key.user
		}
		if key.oldProxy.Valid {
			proxies[strconv.FormatInt(key.oldProxy.Int64, 10)] = strconv.FormatInt(key.proxy.Int64, 10)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, part := range []struct{ table, column, condition string }{
		{"outbox_jobs", "payload", "status IN ('pending','processing','failed')"},
		{"provider_operations", "result_json", "status IN ('queued','processing','partial','pending_review')"},
		{"provider_operation_items", "result_json", "status IN ('queued','processing','pending_review')"},
	} {
		rows, err = tx.QueryContext(ctx, "SELECT rowid,"+quoteSQLName(part.column)+" FROM "+quoteSQLName(part.table)+" WHERE "+part.condition)
		if err != nil {
			return err
		}
		type change struct {
			id  int64
			raw []byte
		}
		var changes []change
		for rows.Next() {
			var id int64
			var raw string
			if err = rows.Scan(&id, &raw); err != nil {
				rows.Close()
				return err
			}
			var value any
			decoder := json.NewDecoder(bytes.NewBufferString(raw))
			decoder.UseNumber()
			if err = decoder.Decode(&value); err != nil {
				rows.Close()
				return err
			}
			if rekeyPayload(value, users, proxies) {
				encoded, e := json.Marshal(value)
				if e != nil {
					rows.Close()
					return e
				}
				changes = append(changes, change{id, encoded})
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, c := range changes {
			if _, err = tx.ExecContext(ctx, "UPDATE "+quoteSQLName(part.table)+" SET "+quoteSQLName(part.column)+"=? WHERE rowid=?", string(c.raw), c.id); err != nil {
				return err
			}
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET target_id=(SELECT CAST(proxy_id AS TEXT) FROM member_identity_aliases WHERE CAST(old_proxy_id AS TEXT)=target_id) WHERE target_type='xnode_user' AND target_id IN (SELECT CAST(old_proxy_id AS TEXT) FROM member_identity_aliases WHERE proxy_id IS NOT NULL)`)
	return err
}

func rekeyPayload(value any, users, proxies map[string]string) bool {
	changed := false
	switch node := value.(type) {
	case []any:
		for _, child := range node {
			changed = rekeyPayload(child, users, proxies) || changed
		}
	case map[string]any:
		for key, child := range node {
			mapping := users
			switch key {
			case "userId", "userID", "user_id", "actorUserId", "ownerUserId", "requestActorId":
			case "proxyUserId", "proxy_user_id":
				mapping = proxies
			default:
				changed = rekeyPayload(child, users, proxies) || changed
				continue
			}
			var raw string
			switch id := child.(type) {
			case string:
				raw = id
			case json.Number:
				raw = string(id)
			default:
				continue
			}
			if next, ok := mapping[raw]; ok && next != raw {
				if _, number := child.(json.Number); number {
					node[key] = json.Number(next)
				} else {
					node[key] = next
				}
				changed = true
			}
		}
	}
	return changed
}
