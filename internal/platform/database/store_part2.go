package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"strings"
	"time"
)

// QueueProxyRepair clears a confirmed missing link without undoing signup.
func (s *Store) QueueProxyRepair(ctx context.Context, userID, missingID string, now time.Time) (model.User, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(missingID) == "" {
		return model.User{}, ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.User{}, fmt.Errorf("begin Proxy repair: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE users SET proxy_user_id=NULL,recovery_reason='',updated_at=?
		WHERE id=? AND onboarding_state='complete' AND proxy_user_id=?`, stamp(now), userID, missingID)
	if err != nil {
		return model.User{}, fmt.Errorf("clear missing Proxy link: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return model.User{}, fmt.Errorf("inspect Proxy repair: %w", err)
	}
	if affected == 1 {
		var purchased int
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=? AND status IN ('active','activating','queued') AND valid_until>?)`, userID, stamp(now)).Scan(&purchased); err != nil {
			return model.User{}, err
		}
		if purchased == 1 {
			if err := insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+userID+`"}`, now, now); err != nil {
				return model.User{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return model.User{}, fmt.Errorf("commit Proxy repair: %w", err)
	}
	return s.UserByID(ctx, userID)
}

// BeginAgreement advances the authenticated member to the published agreements.
func (s *Store) BeginAgreement(ctx context.Context, userID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE users SET onboarding_state='agreement',updated_at=? WHERE id=? AND onboarding_state IN ('intro','membership','username','agreement')`, stamp(time.Now().UTC()), userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected != 1 {
		return ErrConflict
	}
	return err
}

// ReserveUsername is retained only for old repository callers; names are never
// reserved or stored. The public username endpoint has been removed.
func (s *Store) ReserveUsername(ctx context.Context, userID, _ string) error {
	return s.BeginAgreement(ctx, userID)
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
