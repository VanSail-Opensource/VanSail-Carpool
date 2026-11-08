package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// EnqueueAccountPurges recovers the same generation's durable job after restart
// or exhausted delivery. Explicit review failures require operator evidence.
func (s *Store) retryAttempt_EnqueueAccountPurges(ctx context.Context, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	health, err := lockCleanupHealth(ctx, tx, false)
	if err != nil {
		return err
	}
	if !cleanupHealthy(health, now) || s.cleanupAdmins.Load() == nil {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT users.id,users.account_generation FROM users
 JOIN account_cleanup_checks c ON c.user_id=users.id
 JOIN account_cleanup_evidence e ON e.nonce=c.admission_evidence_id
 WHERE users.role='user' AND users.account_status='purging' AND c.bot_epoch=$1 AND e.purge_generation=users.account_generation AND e.completed_at>=$2
 AND NOT EXISTS(SELECT 1 FROM outbox_jobs j WHERE j.kind='account_purge'
 AND j.payload::jsonb->>'userId'=users.id AND j.payload::jsonb->>'generation'=users.account_generation::text
 AND (j.status IN ('pending','processing') OR j.status='failed' AND strpos(COALESCE(j.last_error,''),'review_required:')>0))
 ORDER BY users.id LIMIT 64 FOR UPDATE OF users SKIP LOCKED`, health.Epoch, now.Add(-cleanupFinalTTL))
	if err != nil {
		return err
	}
	type target struct {
		id         string
		generation int64
	}
	var targets []target
	for rows.Next() {
		var item target
		if err = rows.Scan(&item.id, &item.generation); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, item)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	for _, item := range targets {
		valid, err := s.accountCleanupAdmissionTx(ctx, tx, item.id, item.generation, now)
		if err != nil {
			return err
		}
		if !valid {
			continue
		}
		if err := enqueueAccountPurgeTx(ctx, tx, item.id, item.generation, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Caller holds the account row lock, serializing lifecycle admission/recovery.
func enqueueAccountPurgeTx(ctx context.Context, tx *sql.Tx, user string, generation int64, now time.Time) error {
	var id, status, lastError string
	err := tx.QueryRowContext(ctx, `SELECT id,status,COALESCE(last_error,'') FROM outbox_jobs WHERE kind='account_purge'
 AND payload::jsonb->>'userId'=$1 AND payload::jsonb->>'generation'=$2
 AND status IN ('pending','processing','failed')
 ORDER BY CASE status WHEN 'processing' THEN 0 WHEN 'pending' THEN 1 ELSE 2 END,created_at DESC,id LIMIT 1 FOR UPDATE`, user, strconv.FormatInt(generation, 10)).Scan(&id, &status, &lastError)
	if err == nil {
		if status != "failed" || strings.Contains(lastError, "review_required:") {
			return nil
		}
		_, err = tx.ExecContext(ctx, `UPDATE outbox_jobs SET status='pending',attempts=0,available_at=$1,updated_at=$2 WHERE id=$3 AND status='failed'`, stamp(now), stamp(now), id)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	payload, err := json.Marshal(map[string]any{"userId": user, "generation": generation})
	if err != nil {
		return err
	}
	return insertOutboxTx(ctx, tx, "account_purge", string(payload), now, now)
}
