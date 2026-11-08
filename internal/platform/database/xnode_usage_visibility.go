package database

import (
	"context"
	"database/sql"
)

// Display history is disposable; journal cursors and financial evidence are not.
// A completed purge suppresses old hourly generations even after rejoin. A
// changed generation or closed enrollment alone never proves a completed purge.
const visibleUsageMinutesSQL = `WITH incoming AS MATERIALIZED (
 SELECT * FROM unnest($1::text[],$2::text[],$3::text[],$4::text[],$5::text[],$6::timestamptz[],
 $7::bigint[],$8::bigint[],$9::timestamptz[],$10::timestamptz[],$11::text[],$12::text[])
 AS v(owner,mode,reference,period,node,minute,principal,generation,observed_from,observed_through,upload,download)
), scopes AS MATERIALIZED (
 SELECT DISTINCT owner,mode,generation FROM incoming
), visible_scopes AS MATERIALIZED (
 SELECT s.* FROM scopes s JOIN users u ON u.id=s.owner WHERE u.account_status<>'purged'
 AND (s.mode<>'hourly' OR NOT EXISTS(
  SELECT 1 FROM account_cleanup_evidence e JOIN audit_events a
  ON a.id='account-purge-complete:'||s.owner||':'||e.purge_generation::text
  AND a.action='account.purged' AND a.target_type='user' AND a.target_id=s.owner
  WHERE e.user_id=s.owner AND e.purge_generation>s.generation
  AND e.source='final' AND e.qualifying=1
 ))
), eligible AS MATERIALIZED (
 SELECT i.* FROM incoming i JOIN visible_scopes s USING(owner,mode,generation)
), written AS (
 INSERT INTO xnode_usage_minutes AS h(owner_id,billing_mode,reference_id,period_id,node_id,minute,
 proxy_user_id,account_generation,observed_from,observed_through,upload_bytes,download_bytes)
 SELECT owner,mode,reference,period,node,minute,principal,NULLIF(generation,-1),observed_from,
 observed_through,upload::numeric,download::numeric FROM eligible
 ON CONFLICT(owner_id,billing_mode,reference_id,proxy_user_id,period_id,node_id,minute)
 DO UPDATE SET upload_bytes=h.upload_bytes+excluded.upload_bytes,
 download_bytes=h.download_bytes+excluded.download_bytes,
 observed_from=LEAST(h.observed_from,excluded.observed_from),
 observed_through=GREATEST(h.observed_through,excluded.observed_through)
 WHERE h.account_generation IS NOT DISTINCT FROM excluded.account_generation RETURNING 1
) SELECT (SELECT COUNT(*) FROM eligible),(SELECT COUNT(*) FROM written)`

// This read and write stay in the caller's serializable traffic transaction.
// SSI conflicts with the final purge's history DELETE and user update; retries
// rerun eligibility without acquiring a users lock after native period locks.
func writeVisibleUsageMinutesTx(ctx context.Context, tx *sql.Tx, args ...any) error {
	var eligible, written int64
	if err := tx.QueryRowContext(ctx, visibleUsageMinutesSQL, args...).Scan(&eligible, &written); err != nil {
		return err
	}
	if eligible != written {
		return ErrConflict
	}
	return nil
}
