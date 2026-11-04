package database

import (
	"context"
	"time"
)

// ConfirmXNodeFirstSync is admitted only with captured authenticated applied
// membership. Node-wide readiness and arbitrary telemetry cannot call this path.
func (s *Store) ConfirmXNodeFirstSync(ctx context.Context, node string, revision uint64, principals []int64, at time.Time) error {
	if revision == 0 || at.IsZero() || len(principals) > 256 {
		return ErrConflict
	}
	if len(principals) == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE xnode_identities owner SET first_synced_at=$1
 WHERE owner.user_id IS NOT NULL AND owner.first_synced_at IS NULL
 AND EXISTS(SELECT 1 FROM xnode_nodes WHERE id=$2 AND applied_revision>=$3)
 AND owner.user_id IN(
 SELECT user_id FROM xnode_identities WHERE proxy_user_id=ANY($4::bigint[]) AND user_id IS NOT NULL
 UNION SELECT user_id FROM xnode_payg_accounts WHERE proxy_user_id=ANY($4::bigint[]))`, stamp(at), node, revision, principals)
	return err
}
