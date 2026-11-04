package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

func (s *Store) XNodeRemovalPeriods(ctx context.Context, id string) ([]xnode.Period, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT period_id FROM xnode_period_nodes WHERE node_id=$1 AND flushed_sequence IS NULL ORDER BY period_id`, id)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []xnode.Period{}
	for _, key := range keys {
		period, err := s.XNodePeriod(ctx, key)
		if err != nil {
			return nil, err
		}
		out = append(out, period)
	}
	return out, nil
}

func lockRemoval(ctx context.Context, tx *sql.Tx, id string, version int64, finish bool) error {
	var current int64
	if err := tx.QueryRowContext(ctx, `SELECT revision FROM xnode_compile_revision WHERE singleton=1 FOR UPDATE`).Scan(&current); err != nil {
		return err
	}
	if current != version {
		return xnode.ErrConflict
	}
	var enabled int
	var retired sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT enabled,retired_at FROM xnode_nodes WHERE id=$1 FOR UPDATE`, id).Scan(&enabled, &retired); err != nil {
		return err
	}
	if retired.Valid || finish && enabled != 0 {
		return xnode.ErrConflict
	}
	checks := []struct{ query, code string }{
		{`SELECT EXISTS(SELECT 1 FROM xnode_group_bindings WHERE node_id=$1)`, "XNODE_REMOVAL_BINDINGS"},
		{`SELECT EXISTS(SELECT 1 FROM xnode_service_bindings WHERE node_id=$1 AND valid_until::timestamptz>now())`, "XNODE_REMOVAL_SERVICE"},
		{`SELECT EXISTS(SELECT 1 FROM xnode_metered_probes WHERE node_id=$1 AND revoked_at IS NULL AND valid_until::timestamptz>now())`, "XNODE_REMOVAL_SERVICE"},
	}
	if finish {
		checks = append(checks, struct{ query, code string }{`SELECT EXISTS(SELECT 1 FROM xnode_period_nodes WHERE node_id=$1 AND flushed_sequence IS NULL) OR EXISTS(SELECT 1 FROM xnode_traffic_streams WHERE node_id=$1 AND has_gaps)`, "XNODE_REMOVAL_PENDING_ACCOUNTING"})
	}
	for _, check := range checks {
		var blocked bool
		if err := tx.QueryRowContext(ctx, check.query, id).Scan(&blocked); err != nil {
			return err
		}
		if blocked {
			return xnode.RemovalBlocked(check.code)
		}
	}
	return nil
}

func (s *Store) PrepareXNodeRemoval(ctx context.Context, id string, version int64) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockRemoval(ctx, tx, id, version, false); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_nodes SET enabled=0,updated_at=$2 WHERE id=$1`, id, stamp(time.Now().UTC())); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) FinishXNodeRemoval(ctx context.Context, id string, version int64, actor, operation string) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err = lockRemoval(ctx, tx, id, version, true); err != nil {
		return false, err
	}
	historical, err := nodeHasHistory(ctx, tx, id)
	if err != nil {
		return false, err
	}
	now := time.Now().UTC()
	if historical {
		_, err = tx.ExecContext(ctx, `UPDATE xnode_nodes SET retired_at=$2,token_hash=NULL,enabled=0,updated_at=$2 WHERE id=$1`, id, stamp(now))
	} else {
		if _, err = tx.ExecContext(ctx, `DELETE FROM xnode_endpoints WHERE node_id=$1`, id); err != nil {
			return false, err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM xnode_nodes WHERE id=$1`, id)
	}
	if err != nil {
		return false, err
	}
	detail := fmt.Sprintf(`{"retired":%t,"historicalAccounting":"retained"}`, historical)
	if err = insertAuditTx(ctx, tx, "node-removal:"+operation, &actor, "xnode.node_removed", "xnode_node", id, detail, now); err != nil {
		return false, err
	}
	return historical, tx.Commit()
}

// Preserve any referenced history; only endpoint and cascading operational
// children may disappear with a never-enrolled node.
func nodeHasHistory(ctx context.Context, tx *sql.Tx, id string) (bool, error) {
	var used bool
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(agent_id,'')<>'' OR EXISTS(SELECT 1 FROM xnode_traffic_streams WHERE node_id=$1) OR EXISTS(SELECT 1 FROM xnode_imported_history WHERE node_id=$1) FROM xnode_nodes WHERE id=$1`, id).Scan(&used); err != nil {
		return false, err
	}
	if used {
		return true, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT r.relname,a.attname FROM pg_constraint c JOIN pg_class r ON r.oid=c.conrelid JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=c.conkey[1] WHERE c.contype='f' AND c.confrelid='xnode_nodes'::regclass AND c.confdeltype='a' AND r.relname<>'xnode_endpoints'`)
	if err != nil {
		return false, err
	}
	refs := [][2]string{}
	for rows.Next() {
		var ref [2]string
		if err = rows.Scan(&ref[0], &ref[1]); err != nil {
			rows.Close()
			return false, err
		}
		refs = append(refs, ref)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	for _, ref := range refs {
		query := fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE %s=$1)`, quotePGIdentifier(ref[0]), quotePGIdentifier(ref[1]))
		if err = tx.QueryRowContext(ctx, query, id).Scan(&used); err != nil {
			return false, err
		}
		if used {
			return true, nil
		}
	}
	return false, nil
}

func quotePGIdentifier(name string) string { return `"` + name + `"` }
