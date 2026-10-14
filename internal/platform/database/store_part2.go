package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"strings"
	"time"
)

// QueueRemnawaveRepair clears a confirmed missing link without undoing signup.
func (s *Store) QueueRemnawaveRepair(ctx context.Context, userID, missingID string, now time.Time) (model.User, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(missingID) == "" {
		return model.User{}, ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.User{}, fmt.Errorf("begin Remnawave repair: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE users SET remna_user_id=NULL,remna_subscription_url=NULL,recovery_reason='',updated_at=?
		WHERE id=? AND onboarding_state='complete' AND remna_user_id=?`, stamp(now), userID, missingID)
	if err != nil {
		return model.User{}, fmt.Errorf("clear missing Remnawave link: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return model.User{}, fmt.Errorf("inspect Remnawave repair: %w", err)
	}
	if affected == 1 {
		var purchased int
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=? AND status IN ('active','activating','queued') AND valid_until>?)`, userID, stamp(now)).Scan(&purchased); err != nil {
			return model.User{}, err
		}
		if purchased == 1 {
			if err := insertOutboxTx(ctx, tx, "remna_sync_user", `{"userId":"`+userID+`"}`, now, now); err != nil {
				return model.User{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return model.User{}, fmt.Errorf("commit Remnawave repair: %w", err)
	}
	return s.UserByID(ctx, userID)
}

// ReserveUsername atomically assigns a locally unique username.

func (s *Store) ReserveUsername(ctx context.Context, userID, username string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	result, err := s.db.ExecContext(ctx, `UPDATE users SET username=?,onboarding_state='agreement',recovery_reason='',updated_at=?
		WHERE id=? AND (onboarding_state IN ('intro','username') OR (onboarding_state='agreement' AND username=?))`,
		username, stamp(time.Now().UTC()), userID, username)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ErrConflict
		}
		return fmt.Errorf("reserve username: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect username reservation: %w", err)
	}
	if affected == 0 {
		return ErrConflict
	}
	return nil
}

func stamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseStamp(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
