package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Store) ActiveAdminTemporaryBan(ctx context.Context, userID string) (*AdminTemporaryBan, error) {
	var result AdminTemporaryBan
	var expires string
	var restored sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT user_id,actor_user_id,reason,expires_at,COALESCE(unban_operation_id,''),restored_at FROM admin_temporary_bans WHERE user_id=$1 AND restored_at IS NULL`, userID).Scan(&result.UserID, &result.ActorUserID, &result.Reason, &expires, &result.UnbanOperationID, &restored)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if result.ExpiresAt, err = parseStamp(expires); err != nil {
		return nil, err
	}
	result.RestoredAt, err = parseOptionalStamp(restored)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// CompleteAdminTemporaryUnban cannot release a replacement ban for the same user.
func (s *Store) CompleteAdminTemporaryUnban(ctx context.Context, userID, operationID string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE admin_temporary_bans SET restored_at=$1,updated_at=$2
		WHERE user_id=$3 AND unban_operation_id=$4 AND restored_at IS NULL`, stamp(now), stamp(now), userID, operationID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 0 {
		return err
	}
	var replayed bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_temporary_bans
		WHERE user_id=$1 AND unban_operation_id=$2 AND restored_at IS NOT NULL)`, userID, operationID).Scan(&replayed); err != nil {
		return err
	}
	if !replayed {
		return ErrConflict
	}
	return nil
}

// SetAdminTemporaryBanAccess binds native restriction writes to the current
// command and panel owner, so an old delivery cannot overwrite a later ban.
func (s *Store) SetAdminTemporaryBanAccess(ctx context.Context, userID string, proxyUserID int64, operationID string, disabled bool) error {
	for attempt := 0; ; attempt++ {
		err := s.setAdminTemporaryBanAccess(ctx, userID, proxyUserID, operationID, disabled)
		if !s.retryTransaction(ctx, err, attempt) {
			return err
		}
	}
}

func (s *Store) setAdminTemporaryBanAccess(ctx context.Context, userID string, proxyUserID int64, operationID string, disabled bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var banID, unbanID string
	var restored sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT ban_operation_id,COALESCE(unban_operation_id,''),restored_at
		FROM admin_temporary_bans WHERE user_id=$1 FOR UPDATE`, userID).Scan(&banID, &unbanID, &restored); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConflict
		}
		return err
	}
	expected := unbanID
	if disabled {
		expected = banID
	}
	if restored.Valid || operationID == "" || operationID != expected {
		return ErrConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE xnode_identities SET access_disabled=$1
		WHERE proxy_user_id=$2 AND user_id=$3 AND ($4=1 OR EXISTS(SELECT 1 FROM users WHERE id=$3 AND account_status='active'))`,
		boolInt(disabled), proxyUserID, userID, boolInt(disabled))
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return ErrConflict
	}
	return tx.Commit()
}
