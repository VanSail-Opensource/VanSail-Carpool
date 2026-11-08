package database

import (
	"context"
	"database/sql"
	"time"
)

// An attempt marker is not proof that a remote mutation failed. Only an intent
// which never entered execution can be superseded without reconciliation.
const unattemptedAccountOperation = `p.status='queued' AND p.attempts=0 AND p.attempt_started_at IS NULL
 AND COALESCE(p.provider_reference,'')='' AND NOT EXISTS(SELECT 1 FROM provider_operation_items i
 WHERE i.operation_id=p.id AND (i.status<>'queued' OR i.attempt_started_at IS NOT NULL OR COALESCE(i.provider_reference,'')<>''))
 AND NOT (p.kind='emby_setup' AND EXISTS(SELECT 1 FROM emby_accounts a
 WHERE a.user_id=COALESCE(NULLIF(p.owner_user_id,''),p.actor_user_id)
 AND (a.create_attempted=1 OR COALESCE(a.remote_user_id,'')<>'')))`

type accountPurgeReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// AccountPurgeDependency exposes unresolved financial/native outcomes to the
// existing resumable lifecycle. It neither cancels a receipt nor changes funds.
func (s *Store) AccountPurgeDependency(ctx context.Context, user string, _ int64, at time.Time) (string, error) {
	return accountPurgeDependency(ctx, s.db, user, at)
}

func accountPurgeDependency(ctx context.Context, reader accountPurgeReader, user string, at time.Time) (string, error) {
	var pending bool
	err := reader.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM provider_operations p
 WHERE (p.owner_user_id=$1 OR p.actor_user_id=$1) AND p.status IN('queued','processing','pending_review','partial')
 AND NOT (`+unattemptedAccountOperation+`))`, user).Scan(&pending)
	if err != nil || pending {
		return "account_purge_provider_outcome_pending", err
	}
	err = reader.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM emby_accounts WHERE user_id=$1
 AND create_attempted=1 AND COALESCE(remote_user_id,'')='' AND status<>'failed')`, user).Scan(&pending)
	if err != nil || pending {
		return "account_purge_emby_creation_pending", err
	}
	err = reader.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1
 AND status IN('creating','pending') AND (status='creating' OR expires_at='' OR expires_at::timestamptz>$2))`, user, at.UTC()).Scan(&pending)
	if err != nil || pending {
		return "account_purge_payment_outcome_pending", err
	}
	return "", nil
}
