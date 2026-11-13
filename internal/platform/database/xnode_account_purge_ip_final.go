package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Caller already holds cleanup health/user/evidence and has fenced provider
// work. Direct finalization cannot substitute local expiry or a missing row for
// an exact native removal receipt, even if no native periods remain.
func accountPurgeIPControlsReadyTx(ctx context.Context, tx *sql.Tx, owner string, generation int64) error {
	quiet, err := accountPurgeIPMutationsQuiescentTx(ctx, tx, owner)
	if err != nil {
		return err
	}
	if !quiet {
		return fmt.Errorf("ACCOUNT_PURGE_IP_MUTATION_PENDING: %w", xnode.ErrAccessAccountingPending)
	}
	var pending bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM connection_ip_blocks b WHERE b.user_id=$1
 AND NOT EXISTS(SELECT 1 FROM audit_events a JOIN outbox_jobs j ON j.id=a.detail::jsonb->>'jobId'
 JOIN xnode_nodes n ON n.id=b.node_uuid
 WHERE a.action='account.purge_ip_cleared' AND a.target_type='connection_ip_block' AND a.target_id=b.id
 AND j.kind='account_purge' AND j.payload::jsonb->>'userId'=$1 AND j.payload::jsonb->>'generation'=$2
 AND a.detail::jsonb->>'generation'=$2 AND a.detail::jsonb->>'ownerId'=$1
 AND a.detail::jsonb->>'nodeId'=b.node_uuid AND a.detail::jsonb->>'agentId'=n.agent_id
 AND a.detail::jsonb->>'ipDigest'=b.ip_digest AND `+purchaseTimestampOrderSQL("(a.detail::jsonb->>'expiresAt')")+`=`+purchaseTimestampOrderSQL("b.expires_at")+`
 AND a.detail::jsonb->>'proof'='positive_subject_unblock'
 AND a.detail::jsonb->'principals'=(SELECT COALESCE(jsonb_agg(i.proxy_user_id ORDER BY i.proxy_user_id),'[]'::jsonb)
 FROM xnode_identities i WHERE i.user_id=$1
 OR EXISTS(SELECT 1 FROM xnode_payg_accounts p WHERE p.proxy_user_id=i.proxy_user_id AND p.user_id=$1)
 OR EXISTS(SELECT 1 FROM hourly_enrollments e WHERE e.proxy_user_id=i.proxy_user_id AND e.owner_id=$1 AND e.account_generation<$3))
 AND jsonb_array_length(a.detail::jsonb->'principals')>0))`, owner, strconv.FormatInt(generation, 10), generation).Scan(&pending)
	if err != nil {
		return err
	}
	if pending {
		return fmt.Errorf("ACCOUNT_PURGE_IP_EVIDENCE_PENDING: %w", xnode.ErrAccessAccountingPending)
	}
	return nil
}
