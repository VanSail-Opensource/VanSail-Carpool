package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// DeleteXNodeGroupAtVersion rechecks membership and removes only live catalog
// state. The encrypted service/endpoint read is fenced by the compile revision.
func (s *Store) retryAttempt_DeleteXNodeGroupAtVersion(ctx context.Context, id string, version int64, endpoints []xnode.Endpoint) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var serial int64
	if err = tx.QueryRowContext(ctx, `SELECT singleton FROM pricing_worker_watermark WHERE singleton=1 /*pg:FOR UPDATE*/`).Scan(&serial); err != nil {
		return err
	}
	var name string
	if err = tx.QueryRowContext(ctx, `SELECT name FROM xnode_groups WHERE id=?`, id).Scan(&name); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM xnode_compile_revision WHERE singleton=1 /*pg:FOR UPDATE*/`).Scan(&serial); err != nil {
		return err
	}
	if serial != version {
		return xnode.ErrConflict
	}
	now := time.Now().UTC()
	usage, err := readGroupUsage(ctx, tx, now)
	if err != nil {
		return err
	}
	if u := usage[id]; u.Members > 0 || u.PendingAccounting {
		return xnode.ErrConflict
	}
	if err = removeGroupFromCombosTx(ctx, tx, id, now); err != nil {
		return err
	}
	queries := []struct {
		sql  string
		args []any
	}{
		{`UPDATE pricing_admin_overrides SET status='CANCELLED' WHERE status='ACTIVE' AND ((scope_type='product' AND scope_id IN (?,?)) OR (scope_type='user_product' AND (scope_id LIKE ? ESCAPE '\' OR scope_id LIKE ? ESCAPE '\')))`, []any{"addon:" + id, "payg:" + id, "%:addon:" + escapeLike(id), "%:payg:" + escapeLike(id)}},
		{`DELETE FROM pricing_personal_decisions WHERE product_id=? AND product_type IN ('addon','payg')`, []any{id}},
		{`DELETE FROM pricing_product_settings WHERE product_id=? AND product_type IN ('addon','payg')`, []any{id}},
		{`DELETE FROM squad_product_overrides WHERE group_id=?`, []any{id}},
		{`DELETE FROM user_disabled_squads WHERE squad_uuid=?`, []any{id}},
		{`DELETE FROM user_disabled_payg_squads WHERE group_id=?`, []any{id}},
		{`DELETE FROM xnode_groups WHERE id=?`, []any{id}},
	}
	for _, q := range queries {
		if _, err = tx.ExecContext(ctx, q.sql, q.args...); err != nil {
			return err
		}
	}
	for _, item := range endpoints {
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_endpoints SET overrides_cipher=?,updated_at=? WHERE id=?`, item.OverridesCipher, stamp(now), item.ID); err != nil {
			return err
		}
	}
	if err = insertAuditTx(ctx, tx, fmt.Sprintf("group-deleted:%s:%d", id, version), nil, "xnode.group_deleted", "xnode_group", id, `{"historicalAccounting":"retained"}`, now); err != nil {
		return err
	}
	return tx.Commit()
}
