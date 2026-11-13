package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrBackupForbidden means the deletion actor no longer has eligible admin authority.
var ErrBackupForbidden = errors.New("backup deletion requires an eligible administrator")

// Keep the current actor locked through file staging, metadata deletion and audit
// commit. An HTTP authorization snapshot may predate a long-running export.
func requireDeletionActor(ctx context.Context, tx *sql.Tx, actorID string) error {
	if strings.TrimSpace(actorID) == "" {
		return ErrBackupForbidden
	}
	var role, status string
	var blocked bool
	err := tx.QueryRowContext(ctx, `SELECT role,account_status,bot_blocked=1 FROM users WHERE id=$1 FOR SHARE`, actorID).Scan(&role, &status, &blocked)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (role != "admin" || status != "active" || blocked) {
		return ErrBackupForbidden
	}
	if err != nil {
		return fmt.Errorf("verify backup deletion actor: %w", err)
	}
	return nil
}
