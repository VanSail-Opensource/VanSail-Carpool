package database

import (
	"context"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// PutAdminXNodeGlobalAtVersion commits only the authority and compile epoch
// validated before remote work, together with a sanitized settings audit.
func (s *Store) retryAttempt_PutAdminXNodeGlobalAtVersion(ctx context.Context, actor, cipher string, version int64, until time.Time) error {
	if strings.TrimSpace(cipher) == "" {
		return xnode.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return err
	}
	var current int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM xnode_compile_revision WHERE singleton=1 FOR UPDATE`).Scan(&current); err != nil {
		return err
	}
	if current != version {
		return xnode.ErrConflict
	}
	now := time.Now().UTC()
	if _, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at,updated_by)
		VALUES($1,$2,1,$3,$4) ON CONFLICT(key) DO UPDATE SET value=excluded.value,
		encrypted=1,updated_at=excluded.updated_at,updated_by=excluded.updated_by`,
		xnode.GlobalSettingKey, cipher, stamp(now), actor); err != nil {
		return err
	}
	if err = catalogAdminAuditTx(ctx, tx, actor, "setting.update", "setting", xnode.GlobalSettingKey, map[string]bool{"secret": true}, now); err != nil {
		return err
	}
	// Check the actual database clock after every write that may have waited
	// for a lock. A timestamp captured before waiting is not an expiry fence.
	if !until.IsZero() {
		var observed time.Time
		if err = tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&observed); err != nil {
			return err
		}
		if !observed.Before(until) {
			return xnode.ErrConflict
		}
	}
	return tx.Commit()
}
