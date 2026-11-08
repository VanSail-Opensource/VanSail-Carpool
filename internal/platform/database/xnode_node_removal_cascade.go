package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func pruneRemovalDependenciesTx(ctx context.Context, tx *sql.Tx, node string, plans []xnode.RoutingPlan, now time.Time) error {
	for _, plan := range plans {
		result, err := tx.ExecContext(ctx, `UPDATE xnode_routing_plans SET cipher=$1,previous_cipher=$2,phase='disabled',reason=$3,revision=revision+1,updated_at=$4 WHERE proxy_user_id=$5 AND revision=$6 AND operation_id=$7 AND phase=$8`, plan.Cipher, plan.PreviousCipher, "routing_node_removal_pending:"+node, stamp(now), plan.ProxyUserID, plan.Revision, plan.OperationID, plan.Phase)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return xnode.ErrConflict
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM xnode_customizations WHERE proxy_user_id=$1`, plan.ProxyUserID); err != nil {
			return err
		}
	}
	queries := []struct {
		query string
		args  []any
	}{
		{`WITH removed AS (DELETE FROM xnode_group_bindings WHERE node_id=$1 RETURNING group_id) UPDATE xnode_groups g SET enabled=0 WHERE g.id IN (SELECT group_id FROM removed) AND NOT EXISTS(SELECT 1 FROM xnode_group_bindings b WHERE b.group_id=g.id AND b.node_id<>$1)`, []any{node}},
		{`DELETE FROM xnode_service_bindings WHERE node_id=$1`, []any{node}},
		{`UPDATE xnode_identities SET access_disabled=1 WHERE access_disabled=0 AND proxy_user_id IN (SELECT proxy_user_id FROM xnode_metered_probes WHERE node_id=$1)`, []any{node}},
		{`UPDATE xnode_metered_probes SET revoked_at=$2 WHERE node_id=$1 AND revoked_at IS NULL`, []any{node, stamp(now)}},
		{`DELETE FROM xnode_customizations WHERE node_id=$1`, []any{node}},
		{`UPDATE xnode_ddns_changes SET phase='obsolete' WHERE node_id=$1 AND phase IN ('staging','routes')`, []any{node}},
		{`DELETE FROM xnode_ddns_jobs WHERE node_id=$1`, []any{node}},
		{`DELETE FROM xnode_endpoints WHERE node_id=$1`, []any{node}},
		{`UPDATE xnode_nodes SET enabled=0,removing_at=$2,updated_at=$2 WHERE id=$1 AND removing_at IS NULL`, []any{node, stamp(now)}},
	}
	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query.query, query.args...); err != nil {
			return err
		}
	}
	return nil
}

func finishRemovalRoutesTx(ctx context.Context, tx *sql.Tx, node string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE xnode_routing_plans SET previous_cipher='',reason='routing_node_removed',revision=revision+1,updated_at=$2 WHERE reason=$1 AND phase='disabled'`, "routing_node_removal_pending:"+node, stamp(now))
	return err
}
