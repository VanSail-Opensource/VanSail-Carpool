package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func rekeyIdentityCiphertexts(ctx context.Context, tx *sql.Tx, vault *secret.Vault, key identityRekey) error {
	if key.user != "" && key.oldUser != key.user {
		if err := rekeyQueuedSealedTargets(ctx, tx, vault, key); err != nil {
			return err
		}
		for _, item := range []struct{ table, column, context string }{
			{"xnode_identities", "credentials_cipher", "xnode.identity.user."},
			{"xnode_identities", "subscription_cipher", "xnode.subscription.user."},
			{"xnode_member_diy", "cipher", "xnode.diy."},
			{"emby_accounts", "password_ciphertext", "emby.provisioning.password:"},
		} {
			if err := resealColumn(ctx, tx, vault, item.table, item.column, "user_id", key.oldUser, item.context+key.oldUser, item.context+key.user); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE emby_accounts SET password_context=? WHERE user_id=? AND password_context=?`, "emby.provisioning.password:"+key.user, key.oldUser, "emby.provisioning.password:"+key.oldUser); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT rowid,node_id FROM xnode_customizations WHERE user_id=?`, key.oldUser)
		if err != nil {
			return err
		}
		type customizationKey struct {
			id   int64
			node string
		}
		var nodes []customizationKey
		for rows.Next() {
			var node customizationKey
			if err = rows.Scan(&node.id, &node.node); err != nil {
				rows.Close()
				return err
			}
			nodes = append(nodes, node)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, node := range nodes {
			for _, item := range []struct{ column, prefix string }{{"outbounds_cipher", "xnode.outbounds."}, {"routing_cipher", "xnode.routing."}} {
				if err = resealColumn(ctx, tx, vault, "xnode_customizations", item.column, "rowid", node.id, item.prefix+key.oldUser+"."+node.node, item.prefix+key.user+"."+node.node); err != nil {
					return err
				}
			}
		}
	}
	if key.oldProxy.Valid && key.oldProxy.Int64 != key.proxy.Int64 {
		if key.user == "" {
			for _, item := range []struct{ column, prefix string }{{"outbounds_cipher", "xnode.outbounds.native."}, {"routing_cipher", "xnode.routing.native."}} {
				rows, err := tx.QueryContext(ctx, `SELECT rowid,node_id FROM xnode_customizations WHERE proxy_user_id=?`, key.oldProxy.Int64)
				if err != nil {
					return err
				}
				type custom struct {
					id   int64
					node string
				}
				var items []custom
				for rows.Next() {
					var row custom
					if err = rows.Scan(&row.id, &row.node); err != nil {
						rows.Close()
						return err
					}
					items = append(items, row)
				}
				err = rows.Err()
				rows.Close()
				if err != nil {
					return err
				}
				for _, row := range items {
					if err = resealColumn(ctx, tx, vault, "xnode_customizations", item.column, "rowid", row.id, fmt.Sprint(item.prefix, key.oldProxy.Int64, ".", row.node), fmt.Sprint(item.prefix, key.proxy.Int64, ".", row.node)); err != nil {
						return err
					}
				}
			}
		}
		for _, item := range []struct{ column, prefix string }{{"cipher", "xnode.routes."}, {"previous_cipher", "xnode.routes."}, {"legacy_cipher", "xnode.routes.legacy."}} {
			if err := resealColumn(ctx, tx, vault, "xnode_routing_plans", item.column, "proxy_user_id", key.oldProxy.Int64, fmt.Sprint(item.prefix, key.oldProxy.Int64), fmt.Sprint(item.prefix, key.proxy.Int64)); err != nil {
				return err
			}
		}
	}
	return nil
}

func resealColumn(ctx context.Context, tx *sql.Tx, vault *secret.Vault, table, column, where string, id any, before, after string) error {
	var raw string
	err := tx.QueryRowContext(ctx, "SELECT "+quoteSQLName(column)+" FROM "+quoteSQLName(table)+" WHERE "+quoteSQLName(where)+"=?", id).Scan(&raw)
	if err == sql.ErrNoRows || err == nil && raw == "" {
		return nil
	}
	if err != nil {
		return err
	}
	plain, err := vault.Decrypt(before, raw)
	if err != nil {
		return err
	}
	sealed, err := vault.Encrypt(after, plain)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE "+quoteSQLName(table)+" SET "+quoteSQLName(column)+"=? WHERE "+quoteSQLName(where)+"=?", sealed, id)
	return err
}
