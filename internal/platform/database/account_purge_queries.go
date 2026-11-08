package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// AccountPurge reads only the current durable cleanup generation.
func (s *Store) AccountPurge(ctx context.Context, id string, generation int64) (*model.AccountPurge, error) {
	target := &model.AccountPurge{UserID: id, Generation: generation}
	var cutoff string
	err := s.db.QueryRowContext(ctx, `SELECT last_purged_at FROM users WHERE id=$1 AND role='user' AND account_status='purging' AND account_generation=$2`, id, generation).Scan(&cutoff)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if target.Cutoff, err = parseStamp(cutoff); err != nil {
		return nil, err
	}
	var attempted int
	var status string
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(remote_user_id,''),create_attempted,status FROM emby_accounts WHERE user_id=$1`, id).Scan(&target.RemoteEmbyID, &attempted, &status)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	target.EmbyUncertain = target.RemoteEmbyID == "" && attempted == 1 && status != "failed"
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_jobs WHERE kind<>'account_purge' AND status='processing' AND `+accountJobTargets+`)`, id, id, id, id, id).Scan(&target.Busy)
	return target, err
}

// resumeAccountAuthentication starts a new lifecycle after an explicit unblock.
// Canonical IDs and archived financial rows remain stable; privileges do not.
func (s *Store) resumeAccountAuthentication(ctx context.Context, telegramID int64, now time.Time) error {
	var status string
	var blocked bool
	err := s.db.QueryRowContext(ctx, `SELECT account_status,bot_blocked FROM users WHERE telegram_id=$1`, telegramID).Scan(&status, &blocked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if blocked || status == "purging" {
		return ErrAccountUnavailable
	}
	if status == "purged" {
		result, err := s.db.ExecContext(ctx, `UPDATE users SET account_status='active',onboarding_state='intro',policy_accepted_at=NULL,accepted_agreement_revision=0,created_at=$1,updated_at=$2 WHERE telegram_id=$3 AND account_status='purged' AND bot_blocked=0`, stamp(now), stamp(now), telegramID)
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			return ErrAccountUnavailable
		}
	}
	return nil
}
