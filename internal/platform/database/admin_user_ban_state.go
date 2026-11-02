package database

import (
	"context"
	"database/sql"
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

func (s *Store) CompleteAdminTemporaryUnban(ctx context.Context, userID string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_temporary_bans SET restored_at=$1,updated_at=$2 WHERE user_id=$3 AND restored_at IS NULL`, stamp(now), stamp(now), userID)
	return err
}
