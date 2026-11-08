package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) admitCleanupTx(ctx context.Context, tx *sql.Tx, p model.AccountCleanupProbe, o model.AccountCleanupObservation, c cleanupCheck, current int64, status string, at time.Time) (model.AccountCleanupCompletion, error) {
	result := model.AccountCleanupCompletion{Accepted: true, Generation: current}
	if status != "active" && status != "purging" {
		result.Reason = "account_superseded"
		return result, nil
	}
	if status == "active" {
		if current == math.MaxInt64 {
			return result, ErrConflict
		}
		current++
		if _, err := tx.ExecContext(ctx, `UPDATE users SET account_status='purging',account_generation=$2,last_purged_at=$3,onboarding_state='intro',updated_at=$3 WHERE id=$1`, p.UserID, current, stamp(at)); err != nil {
			return result, err
		}
	}
	if !c.admission.Valid {
		if err := fenceCleanupAccessTx(ctx, tx, p.UserID, at); err != nil {
			return result, err
		}
	}
	if err := insertCleanupEvidence(ctx, tx, p, o, true, current, "final", at); err != nil {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE account_cleanup_checks SET admission_evidence_id=$2,pending_nonce=NULL,pending_started_at=NULL,pending_final=0,next_probe_at=$3 WHERE user_id=$1`, p.UserID, p.Nonce, at.Add(4*time.Minute)); err != nil {
		return result, err
	}
	// A dispatcher may already hold an outbox claim and have observed active
	// state. Serialize its BeginAttempt CAS before this admission commits.
	if err := fenceAccountPurgeOperationsTx(ctx, tx, p.UserID, at); err != nil {
		return result, err
	}
	if err := enqueueAccountPurgeTx(ctx, tx, p.UserID, current, at); err != nil {
		return result, err
	}
	// A fresh final check wakes only this same canonical pending generation.
	if _, err := tx.ExecContext(ctx, `UPDATE outbox_jobs SET available_at=LEAST(available_at,$3),updated_at=$3 WHERE kind='account_purge' AND status='pending' AND payload::jsonb->>'userId'=$1 AND payload::jsonb->>'generation'=$2`, p.UserID, strconv.FormatInt(current, 10), stamp(at)); err != nil {
		return result, err
	}
	meta, err := json.Marshal(map[string]any{"generation": current, "evidence": p.Nonce})
	if err != nil {
		return result, err
	}
	if err = insertAuditTx(ctx, tx, "account-cleanup-admission:"+p.Nonce, nil, "account.cleanup_admitted", "user", p.UserID, string(meta), at); err != nil {
		return result, err
	}
	result.Admitted = true
	result.Generation = current
	result.Reason = "admitted"
	return result, nil
}

func fenceCleanupAccessTx(ctx context.Context, tx *sql.Tx, id string, at time.Time) error {
	queries := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM sessions WHERE user_id=$1`, []any{id}},
		{`DELETE FROM browser_access_links WHERE user_id=$1`, []any{id}},
		{`UPDATE xnode_identities SET access_disabled=1,subscription_hash=NULL,subscription_cipher='' WHERE user_id=$1 OR proxy_user_id IN(SELECT proxy_user_id FROM xnode_payg_accounts WHERE user_id=$1 UNION SELECT proxy_user_id FROM hourly_enrollments WHERE owner_id=$1)`, []any{id}},
		{`UPDATE purchases SET status='cancelled',auto_renew_enabled=0,updated_at=$2 WHERE user_id=$1 AND status IN('active','activating','queued')`, []any{id, stamp(at)}},
		{`UPDATE hourly_enrollments SET status='closed',updated_at=$2 WHERE owner_id=$1 AND status IN('preparing','active','funding_suspended','disabled')`, []any{id, at.UTC()}},
		{`UPDATE payment_orders SET cancelled_at=COALESCE(cancelled_at,$2),cancel_reason='ACCOUNT_PURGED',payment_url=NULL,qr_payload=NULL WHERE user_id=$1 AND status IN('creating','pending')`, []any{id, stamp(at)}},
	}
	for _, q := range queries {
		if _, err := tx.ExecContext(ctx, q.sql, q.args...); err != nil {
			return err
		}
	}
	return cancelAccountQueuedJobsTx(ctx, tx, id)
}
