package database

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const accountPurgeIPUnproven = `user_id=$1 AND NOT EXISTS(SELECT 1 FROM audit_events a
 WHERE a.action='account.purge_ip_cleared' AND a.target_type='connection_ip_block' AND a.target_id=connection_ip_blocks.id
 AND a.detail::jsonb->>'jobId'=$2 AND a.detail::jsonb->>'generation'=$3
 AND a.detail::jsonb->>'ownerId'=user_id AND a.detail::jsonb->>'nodeId'=node_uuid
 AND a.detail::jsonb->>'ipDigest'=ip_digest AND (a.detail::jsonb->>'expiresAt')::timestamptz=expires_at::timestamptz)`

// AccountPurgeIPControls retains all encrypted rows. Only an exact durable
// generation-bound receipt removes a row from this barrier's pending view.
func (s *Store) AccountPurgeIPControls(ctx context.Context, target model.AccountPurge, jobID string) ([]xnode.AccountPurgeIPControl, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	_, allowed, err := s.accountNativePurgeGuardTx(ctx, tx, target, jobID)
	if err != nil || !allowed {
		return nil, false, err
	}
	var cursor, kind string
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(payload::jsonb->>'nativeIPCursor',''),
 COALESCE(jsonb_typeof(payload::jsonb->'nativeIPCursor'),'string') FROM outbox_jobs WHERE id=$1`, jobID).Scan(&cursor, &kind)
	if err != nil || kind != "string" || len(cursor) > 512 {
		if err == nil {
			err = xnode.ErrInvalid
		}
		return nil, false, err
	}
	controls, err := accountPurgeIPPageTx(ctx, tx, target, jobID, cursor)
	if err == nil && len(controls) == 0 && cursor != "" {
		controls, err = accountPurgeIPPageTx(ctx, tx, target, jobID, "")
	}
	if err != nil {
		return nil, false, err
	}
	principals, err := accountPurgeIPPrincipalsTx(ctx, tx, target)
	if err != nil {
		return nil, false, err
	}
	var pending bool
	quiet, err := accountPurgeIPMutationsQuiescentTx(ctx, tx, target.UserID)
	pending = !quiet
	if err != nil {
		return nil, false, err
	}
	for i := range controls {
		controls[i].Principals, controls[i].MutationPending = principals, pending
		var agent string
		err = tx.QueryRowContext(ctx, `SELECT agent_id FROM xnode_nodes WHERE id=$1`, controls[i].Block.NodeUUID).Scan(&agent)
		if err != nil && err != sql.ErrNoRows {
			return nil, false, err
		}
		controls[i].AgentID = agent
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return controls, true, nil
}

func accountPurgeIPPageTx(ctx context.Context, tx *sql.Tx, target model.AccountPurge, jobID, cursor string) ([]xnode.AccountPurgeIPControl, error) {
	rows, err := tx.QueryContext(ctx, ipBlockSelect+` WHERE `+accountPurgeIPUnproven+` AND id>$4 ORDER BY id LIMIT 128`, target.UserID, jobID, strconv.FormatInt(target.Generation, 10), cursor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	controls := make([]xnode.AccountPurgeIPControl, 0, 128)
	for rows.Next() {
		block, e := scanIPBlock(rows)
		if e != nil {
			return nil, e
		}
		controls = append(controls, xnode.AccountPurgeIPControl{Block: block})
	}
	return controls, rows.Err()
}

func accountPurgeIPPrincipalsTx(ctx context.Context, tx *sql.Tx, target model.AccountPurge) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT proxy_user_id FROM xnode_identities i WHERE i.user_id=$1
 OR EXISTS(SELECT 1 FROM xnode_payg_accounts a WHERE a.proxy_user_id=i.proxy_user_id AND a.user_id=$1)
 OR EXISTS(SELECT 1 FROM hourly_enrollments e WHERE e.proxy_user_id=i.proxy_user_id AND e.owner_id=$1 AND e.account_generation<=$2)
 ORDER BY proxy_user_id LIMIT 8193`, target.UserID, target.ThroughGeneration)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		if id <= 0 || id >= 1<<53 {
			return nil, xnode.ErrInvalid
		}
		ids = append(ids, id)
	}
	if len(ids) > 8192 {
		return nil, xnode.ErrUnavailable
	}
	return ids, rows.Err()
}

func (s *Store) AdvanceAccountPurgeIPControl(ctx context.Context, target model.AccountPurge, jobID, blockID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	_, allowed, err := s.accountNativePurgeGuardTx(ctx, tx, target, jobID)
	if err != nil || !allowed {
		return false, err
	}
	var owned bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM connection_ip_blocks WHERE id=$1 AND user_id=$2)`, blockID, target.UserID).Scan(&owned); err != nil {
		return false, err
	}
	if !owned {
		return false, xnode.ErrConflict
	}
	_, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET payload=jsonb_set(payload::jsonb,'{nativeIPCursor}',to_jsonb($2::text),true)::text WHERE id=$1`, jobID, blockID)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
