package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
)

// AcceptAffiliateReferral freezes a valid inviter on a private /start stub.
func (s *Store) retryAttempt_AcceptAffiliateReferral(ctx context.Context, inviteeTelegramID, inviterTelegramID int64, now time.Time) (string, bool, error) {
	if inviteeTelegramID <= 0 || inviteeTelegramID >= 1<<52 || inviterTelegramID <= 0 || inviteeTelegramID == inviterTelegramID {
		return "", false, affiliates.ErrInvalidInput
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback() }()
	var display string
	var showUsername bool
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(NULLIF(telegram_username,''),NULLIF(telegram_first_name,''),'')
		,COALESCE(p.show_referral_username,1) FROM users LEFT JOIN user_preferences p ON p.user_id=users.id
		WHERE telegram_id=? AND new_user=0`, inviterTelegramID).Scan(&display, &showUsername)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	stubID := strconv.FormatInt(inviteeTelegramID, 10)
	result, err := tx.ExecContext(ctx, `INSERT INTO users(id,telegram_id,new_user,inviter_id,notification_locale,auto_traffic_reset_enabled,created_at,updated_at)
		VALUES(?,?,1,?,'en',1,?,?) ON CONFLICT(telegram_id) DO UPDATE SET inviter_id=excluded.inviter_id,updated_at=excluded.updated_at
		WHERE users.new_user=1 AND users.inviter_id IS NULL`, stubID, inviteeTelegramID, inviterTelegramID, stamp(now), stamp(now))
	if err != nil {
		return "", false, fmt.Errorf("freeze affiliate inviter: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return "", false, err
	}
	if err := tx.Commit(); err != nil {
		return "", false, err
	}
	if !showUsername {
		display = ""
	}
	return display, affected == 1, nil
}
