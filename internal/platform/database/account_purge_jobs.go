package database

import (
	"context"
	"database/sql"
	"time"
)

// FenceAccountPurge serializes pre-attempt supersession with operation workers.
// The generation's accepted grace evidence always precedes any local mutation.
func (s *Store) FenceAccountPurge(ctx context.Context, user string, generation int64, at time.Time) error {
	_, err := hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (struct{}, error) {
		valid, err := s.accountCleanupAdmissionTx(ctx, tx, user, generation, at)
		if err != nil {
			return struct{}{}, err
		}
		if !valid {
			return struct{}{}, ErrCleanupAdmissionRequired
		}
		return struct{}{}, fenceAccountPurgeOperationsTx(ctx, tx, user, at)
	})
	return err
}

func fenceAccountPurgeOperationsTx(ctx context.Context, tx *sql.Tx, user string, at time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT p.id FROM provider_operations p WHERE (p.owner_user_id=$1 OR p.actor_user_id=$1)
 AND `+unattemptedAccountOperation+` ORDER BY p.id FOR UPDATE OF p`, user)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
	}
	if err = finishCompileRows(rows); err != nil {
		return err
	}
	if err = lockAccountPurgeEmbyTx(ctx, tx, user); err != nil {
		return err
	}
	if err = cancelAccountKnownJobsTx(ctx, tx, user); err != nil {
		return err
	}
	if err = supersedeUnattemptedAccountOperationsTx(ctx, tx, user, stamp(at)); err != nil {
		return err
	}
	// A recorded never-attempted checkout has no external payment to cancel.
	// Opaque legacy creating orders retain their unresolved dependency instead.
	_, err = tx.ExecContext(ctx, `UPDATE payment_orders o SET status='failed',cancelled_at=COALESCE(cancelled_at,$2),
 cancel_reason='ACCOUNT_PURGED',payment_url=NULL,qr_payload=NULL,updated_at=$2
 WHERE o.user_id=$1 AND o.status='creating' AND EXISTS(SELECT 1 FROM provider_operations p
 JOIN provider_operation_items i ON i.operation_id=p.id AND i.target_type='payment_order' AND i.target_id=o.id
 WHERE p.kind='payment_create' AND p.status='failed' AND p.error_code='ACCOUNT_PURGED' AND p.attempts=0
 AND p.attempt_started_at IS NULL AND COALESCE(p.provider_reference,'')='')`, user, stamp(at))
	return err
}

// Preserve attempted continuations and their sealed targets. Removing their
// outbox rows would strand an unknown provider result behind the purge fence.
func cancelAccountKnownJobsTx(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE status<>'processing'
 AND kind IN('xnode_access_repair','xnode_apply_entitlement','xnode_sync_user','xnode_prepare_continuity',
 'xnode_payg_reconcile','xnode_payg_recover','emby_provision_account','provider_operation','user_notification')
 AND `+accountJobTargets+`
 AND (kind<>'provider_operation' OR payload::jsonb->>'operationId' IN
 (SELECT p.id FROM provider_operations p WHERE `+unattemptedAccountOperation+`))
 AND (kind<>'emby_provision_account' OR NOT EXISTS
 (SELECT 1 FROM emby_accounts WHERE user_id=$1 AND (create_attempted=1 OR COALESCE(remote_user_id,'')<>'')))`, id, id, id, id, id)
	return err
}

func supersedeUnattemptedAccountOperationsTx(ctx context.Context, tx *sql.Tx, user, now string) error {
	// Both updates use exactly the same pre-attempt predicate. Updating items
	// first would invalidate it, so capture the operation IDs within one query.
	_, err := tx.ExecContext(ctx, `WITH candidates AS MATERIALIZED (
 SELECT p.id FROM provider_operations p WHERE (p.owner_user_id=$1 OR p.actor_user_id=$1)
 AND `+unattemptedAccountOperation+` FOR UPDATE
 ), items AS (UPDATE provider_operation_items SET status='failed',error_code='ACCOUNT_PURGED',
 completed_at=$2,updated_at=$2 WHERE operation_id IN(SELECT id FROM candidates) RETURNING operation_id)
 UPDATE provider_operations SET status='failed',error_code='ACCOUNT_PURGED',completed_at=$2,updated_at=$2
 WHERE id IN(SELECT id FROM candidates)`, user, now)
	return err
}
