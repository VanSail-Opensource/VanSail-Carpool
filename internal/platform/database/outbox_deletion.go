package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// These expiry jobs are the durable backstop for removing an actual restriction.
// A failed delivery does not prove that the restriction has been lifted. Keep
// the exact current generation; superseded or already restored bans may retire.
const requiredRestrictionExpiryJob = `(EXISTS(SELECT 1 FROM connection_ip_blocks block
 WHERE block.expiry_job_id=job.id)
 OR (job.kind='admin_temporary_ban_expiry' AND EXISTS(SELECT 1 FROM admin_temporary_bans ban
 WHERE ban.restored_at IS NULL AND ban.user_id=(CASE WHEN COALESCE(job.payload IS JSON,false) THEN job.payload ELSE '{}' END)::jsonb->>'userId'
 AND (COALESCE((CASE WHEN COALESCE(job.payload IS JSON,false) THEN job.payload ELSE '{}' END)::jsonb->>'banOperationId','')=''
 OR ban.ban_operation_id=(CASE WHEN COALESCE(job.payload IS JSON,false) THEN job.payload ELSE '{}' END)::jsonb->>'banOperationId'))))`

// Historical paid setups still dispatch through their account job. An uncertain
// remote create must retain its retry path until setup completes or is refunded.
const requiredLegacyProvisionJob = `(job.kind='emby_provision_account' AND EXISTS(
 SELECT 1 FROM emby_accounts account WHERE account.id=(CASE WHEN COALESCE(job.payload IS JSON,false) THEN job.payload ELSE '{}' END)::jsonb->>'accountId'
 AND account.status IN ('queued','provisioning','pending_review') AND account.refunded_at IS NULL))`

func outboxDeletionRequiresRecoveryTx(ctx context.Context, tx *sql.Tx, jobID string, now time.Time) (bool, error) {
	var required bool
	err := tx.QueryRowContext(ctx, `SELECT
	 (job.status<>'done' AND job.kind IN ('xnode_hourly_settle','draw_raffle_settle','account_purge','account_cleanup_probe'))
 OR `+requiredRestrictionExpiryJob+`
 OR `+requiredLegacyProvisionJob+`
 OR EXISTS(SELECT 1 FROM provider_operation_items item JOIN provider_operations operation ON operation.id=item.operation_id
 WHERE item.target_type='outbox_job' AND item.target_id=job.id AND operation.kind='outbox_retry'
 AND operation.status IN ('queued','processing','pending_review','partial'))
 OR (job.kind='provider_operation' AND EXISTS(SELECT 1 FROM provider_operations operation
 WHERE operation.id=(CASE WHEN COALESCE(job.payload IS JSON,false) THEN job.payload ELSE '{}' END)::jsonb->>'operationId'
 AND (NOT (`+settledProviderOperation+`) OR (operation.status='failed' AND (`+liveComboControlPredicate(2)+`)))))
 FROM outbox_jobs job WHERE job.id=$1`, jobID, stamp(now), stamp(now), stamp(now), stamp(now)).Scan(&required)
	return required, err
}

func auditOutboxDeletionTx(ctx context.Context, tx *sql.Tx, actorID, jobID, kind, status string, now time.Time) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	details, err := json.Marshal(map[string]string{"kind": kind, "status": status})
	if err != nil {
		return err
	}
	return insertAuditTx(ctx, tx, id, &actorID, "outbox.delete", "outbox_job", jobID, string(details), now)
}
