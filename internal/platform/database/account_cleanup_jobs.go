package database

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"
)

// Health/user/check locks serialize scheduling. A queued nonce can be refreshed
// under the pending row lock; processing work retains its original nonce and
// exact claim. This bounds live probe jobs without adding a competing queue.
func cleanupProbeJobTx(ctx context.Context, tx *sql.Tx, user string, generation int64) (string, bool, error) {
	var id, status string
	err := tx.QueryRowContext(ctx, `SELECT id,status FROM outbox_jobs WHERE kind='account_cleanup_probe' AND payload::jsonb->>'userId'=$1 AND payload::jsonb->>'generation'=$2 AND status IN('pending','processing') ORDER BY CASE status WHEN 'processing' THEN 0 ELSE 1 END,created_at,id LIMIT 1 FOR UPDATE`, user, strconv.FormatInt(generation, 10)).Scan(&id, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return id, status == "processing", err
}

func writeCleanupProbeJobTx(ctx context.Context, tx *sql.Tx, id, body string, at time.Time) error {
	if id == "" {
		return insertOutboxTx(ctx, tx, "account_cleanup_probe", body, at, at)
	}
	_, err := tx.ExecContext(ctx, `UPDATE outbox_jobs SET payload=$2,available_at=LEAST(available_at,$3),updated_at=$3 WHERE id=$1 AND status='pending'`, id, body, stamp(at))
	return err
}
