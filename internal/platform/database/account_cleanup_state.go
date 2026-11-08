package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const (
	cleanupBotTTL   = 15 * time.Minute
	cleanupFinalTTL = 5 * time.Minute
	cleanupProbeTTL = 2 * time.Minute
	cleanupGrace    = 7 * 24 * time.Hour
)

var ErrCleanupAdmissionRequired = errors.New("account cleanup admission evidence is required")

// ErrAccountCleanupRecoveryPending is a verified identity returning to an
// already partially purged lifecycle. It is not invalid Telegram launch data.
var ErrAccountCleanupRecoveryPending = fmt.Errorf("%w: account cleanup recovery is pending", ErrAccountUnavailable)

type cleanupCheck struct {
	user                                    string
	generation, epoch, count                int64
	next                                    time.Time
	reachable, health, first, last, started sql.NullTime
	nonce, admission                        sql.NullString
	final                                   bool
}

const cleanupCheckSelect = `SELECT user_id,account_generation,bot_epoch,next_probe_at,private_reachable_at,health_observed_at,first_failure_at,last_failure_at,failure_count,pending_nonce,pending_started_at,pending_final,admission_evidence_id FROM account_cleanup_checks`

func readCleanupCheck(ctx context.Context, tx *sql.Tx, id string) (cleanupCheck, error) {
	var c cleanupCheck
	err := tx.QueryRowContext(ctx, cleanupCheckSelect+` WHERE user_id=$1 FOR UPDATE`, id).Scan(&c.user, &c.generation, &c.epoch, &c.next, &c.reachable, &c.health, &c.first, &c.last, &c.count, &c.nonce, &c.started, &c.final, &c.admission)
	return c, err
}

func lockCleanupHealth(ctx context.Context, tx *sql.Tx, exclusive bool) (model.AccountCleanupHealth, error) {
	var h model.AccountCleanupHealth
	var verified, paused sql.NullTime
	suffix := " FOR SHARE"
	if exclusive {
		suffix = " FOR UPDATE"
	}
	err := tx.QueryRowContext(ctx, `SELECT bot_id,bot_epoch,verified,verified_at,paused_until FROM account_cleanup_health WHERE singleton=1`+suffix).Scan(&h.BotID, &h.Epoch, &h.Verified, &verified, &paused)
	if verified.Valid {
		h.VerifiedAt = &verified.Time
	}
	if paused.Valid {
		h.PausedUntil = &paused.Time
	}
	return h, err
}

func cleanupVerified(h model.AccountCleanupHealth, at time.Time) bool {
	return h.Verified && h.BotID > 0 && h.Epoch > 0 && h.VerifiedAt != nil && !at.Before(*h.VerifiedAt) && at.Sub(*h.VerifiedAt) <= cleanupBotTTL
}

func cleanupHealthy(h model.AccountCleanupHealth, at time.Time) bool {
	return cleanupVerified(h, at) && (h.PausedUntil == nil || !at.Before(*h.PausedUntil))
}

func cleanupMature(c cleanupCheck, at time.Time) bool {
	return c.count >= 3 && c.first.Valid && c.last.Valid && c.last.Time.Sub(c.first.Time) >= cleanupGrace && !at.Before(c.last.Time)
}
