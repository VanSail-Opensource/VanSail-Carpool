package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func quoteSQLName(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

type identityReference struct {
	table, column string
	proxy         bool
}

func identityReferences(ctx context.Context, tx *sql.Tx) ([]identityReference, error) {
	rows, err := tx.QueryContext(ctx, `SELECT m.name,f."from",f."table",f."to" FROM sqlite_master m JOIN pragma_foreign_key_list(m.name) f WHERE m.type='table' AND f."table" IN ('users','xnode_identities')`)
	if err != nil {
		return nil, err
	}
	refs := []identityReference{{"users", "id", false}, {"xnode_identities", "proxy_user_id", true}}
	seen := map[string]bool{}
	for rows.Next() {
		var table, column, target, to string
		if err = rows.Scan(&table, &column, &target, &to); err != nil {
			rows.Close()
			return nil, err
		}
		ref := identityReference{table, column, target == "xnode_identities"}
		refs = append(refs, ref)
		seen[table+"."+column] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT m.name,p.name FROM sqlite_master m JOIN pragma_table_info(m.name) p WHERE m.type='table' AND m.name<>'member_identity_aliases' AND p.name IN ('user_id','actor_user_id','owner_user_id','proxy_user_id','request_actor_id')`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var table, column string
		if err = rows.Scan(&table, &column); err != nil {
			rows.Close()
			return nil, err
		}
		if !seen[table+"."+column] {
			refs = append(refs, identityReference{table, column, column == "proxy_user_id"})
			seen[table+"."+column] = true
		}
	}
	err = rows.Err()
	rows.Close()
	return refs, err
}

func rekeyIdentityReferences(ctx context.Context, tx *sql.Tx) error {
	refs, err := identityReferences(ctx, tx)
	if err != nil {
		return err
	}
	// Temporary disjoint keys make swaps/collisions safe under UNIQUE indexes.
	for _, stage := range []bool{true, false} {
		for _, ref := range refs {
			before, after, guard := "old_user_id", "'~tg:'||user_id", "user_id<>''"
			if !stage {
				before, after = "'~tg:'||user_id", "user_id"
			}
			if ref.proxy {
				before, after, guard = "old_proxy_id", "-proxy_id", "proxy_id IS NOT NULL"
				if !stage {
					before, after = "-proxy_id", "proxy_id"
				}
			}
			column := quoteSQLName(ref.column)
			query := fmt.Sprintf(`UPDATE %s SET %s=(SELECT %s FROM member_identity_aliases WHERE %s=%s.%s AND %s) WHERE %s IN (SELECT %s FROM member_identity_aliases WHERE %s)`, quoteSQLName(ref.table), column, after, before, quoteSQLName(ref.table), column, guard, column, before, guard)
			if !stage {
				expression := "SUBSTR(" + column + ",5)"
				if ref.proxy {
					expression = "-" + column
				}
				query = fmt.Sprintf(`UPDATE %s SET %s=%s WHERE %s IN (SELECT %s FROM member_identity_aliases WHERE %s)`, quoteSQLName(ref.table), column, expression, column, before, guard)
			}
			if _, err = tx.ExecContext(ctx, query); err != nil {
				return fmt.Errorf("rekey %s.%s: %w", ref.table, ref.column, err)
			}
		}
	}
	return rekeyQueuedIdentityPayloads(ctx, tx)
}
