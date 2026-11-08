package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// Caller holds the users row first. No health lock is acquired on auth paths.
// Empty/healthy accounts require no per-request last-activity write.
func cancelCleanupCandidateTx(ctx context.Context, tx *sql.Tx, id, source string, at time.Time) error {
	c, err := readCleanupCheck(ctx, tx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if c.count == 0 && !c.nonce.Valid && !c.admission.Valid {
		return nil
	}
	var bot int64
	err = tx.QueryRowContext(ctx, `SELECT bot_id FROM account_cleanup_health WHERE singleton=1 AND bot_epoch=$1`, c.epoch).Scan(&bot)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if bot > 0 {
		nonce, e := ids.New()
		if e != nil {
			return e
		}
		p := model.AccountCleanupProbe{UserID: id, AccountGeneration: c.generation, BotID: bot, Epoch: c.epoch, Nonce: nonce, StartedAt: at}
		if e = insertCleanupEvidence(ctx, tx, p, model.AccountCleanupObservation{SendAction: "unknown", GetChat: "unknown"}, false, nil, source, at); e != nil {
			return e
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE account_cleanup_checks SET first_failure_at=NULL,last_failure_at=NULL,failure_count=0,pending_nonce=NULL,pending_started_at=NULL,pending_final=0,admission_evidence_id=NULL,next_probe_at=$2 WHERE user_id=$1`, id, at.Add(24*time.Hour))
	return err
}
