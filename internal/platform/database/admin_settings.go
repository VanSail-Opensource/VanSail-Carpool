package database

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// PutAdminSetting commits the setting and actor audit together without logging values.
func (s *Store) retryAttempt_PutAdminSetting(ctx context.Context, key, value string, encrypted bool, actorID string, now time.Time) error {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return ErrConflict
	}
	auditID, err := ids.New()
	if err != nil {
		return err
	}
	detail, err := json.Marshal(map[string]bool{"secret": encrypted})
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at,updated_by)
		VALUES($1,$2,$3,$4,$5) ON CONFLICT(key) DO UPDATE SET value=excluded.value,
		encrypted=excluded.encrypted,updated_at=excluded.updated_at,updated_by=excluded.updated_by`,
		key, value, boolInt(encrypted), stamp(now), actorID); err != nil {
		return err
	}
	if err := insertAuditTx(ctx, tx, auditID, &actorID, "setting.update", "setting", key, string(detail), now); err != nil {
		return err
	}
	return tx.Commit()
}
