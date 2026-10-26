package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func outboxResourcesTx(ctx context.Context, tx *sql.Tx, job model.OutboxJob) ([]string, error) {
	var payload map[string]json.RawMessage
	_ = json.Unmarshal([]byte(job.Payload), &payload) // Invalid jobs still reach their typed handler.
	value := func(key string) string { var s string; _ = json.Unmarshal(payload[key], &s); return s }
	if job.Kind == "pricing_compute" {
		return []string{"compute:" + value("inputId")}, nil
	}
	keys := []string{}
	add := func(kind, id string) {
		if id != "" {
			keys = append(keys, kind+":"+id)
		}
	}
	add("user", value("userId"))
	add("node", value("nodeId"))
	if job.Kind == "xnode_firewall" { // Dynamic rules must bypass a slow node command.
		keys = []string{"firewall:" + value("nodeId") + ":" + value("bank")}
	}
	for field, table := range map[string]string{"purchaseId": "purchases", "accountId": "emby_accounts", "orderId": "payment_orders"} {
		id := value(field)
		if id == "" {
			continue
		}
		add(field, id)
		var user string
		err := tx.QueryRowContext(ctx, `SELECT user_id FROM `+table+` WHERE id=?`, id).Scan(&user)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		add("user", user)
	}
	if id := value("chatId"); id != "" {
		add("chat", id)
	} else if raw := payload["chatId"]; len(raw) > 0 {
		add("chat", string(raw))
	}
	if id := value("operationId"); id != "" {
		add("operation", id)
		rows, err := tx.QueryContext(ctx, `SELECT 'user',owner_user_id FROM provider_operations WHERE id=? AND owner_user_id IS NOT NULL
   UNION ALL SELECT target_type,target_id FROM provider_operation_items WHERE operation_id=?`, id, id)
		if err != nil {
			return nil, err
		}
		type target struct{ kind, id string }
		targets := []target{}
		for rows.Next() {
			var t target
			if err = rows.Scan(&t.kind, &t.id); err != nil {
				rows.Close()
				return nil, err
			}
			targets = append(targets, t)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		for _, t := range targets {
			switch t.kind {
			case "user", "member_diy", "member_whitelist", "emby_user":
				add("user", t.id)
			case "xnode":
				add("node", t.id)
			case "purchase", "emby_account", "xnode_user":
				query := map[string]string{"purchase": "SELECT user_id FROM purchases WHERE id=?", "emby_account": "SELECT user_id FROM emby_accounts WHERE id=?", "xnode_user": "SELECT user_id FROM xnode_identities WHERE proxy_user_id=?"}[t.kind]
				var user sql.NullString // Standalone native identities have no member owner.
				err := tx.QueryRowContext(ctx, query, t.id).Scan(&user)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return nil, err
				}
				add("user", user.String)
				add(t.kind, t.id)
			default:
				add(t.kind, t.id)
			}
		}
	}
	if strings.HasPrefix(job.Kind, "ip_lookup_") {
		add("lane", "ip-lookup-repair")
	}
	if len(keys) == 0 {
		add("kind", job.Kind)
	}
	return keys, nil
}
