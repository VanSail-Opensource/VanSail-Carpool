package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type identityRekey struct {
	oldUser, user   string
	oldProxy, proxy sql.NullInt64
}

// MigrateTelegramIdentities is an atomic, restart-safe startup migration. It must
// complete before workers start; ciphertext and all references commit together.
func (s *Store) MigrateTelegramIdentities(ctx context.Context, vault *secret.Vault, backupBase ...string) (result error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var done int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('users') WHERE name='username'`).Scan(&done); err != nil || done == 0 {
		return err
	}
	if len(backupBase) > 0 {
		var users int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
			return err
		}
		if users > 0 {
			path := backupBase[0] + ".pre-telegram-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".db"
			if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
				return err
			}
			if err := os.Chmod(path, 0o600); err != nil {
				return err
			}
		}
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, err := conn.ExecContext(cleanup, `PRAGMA legacy_alter_table=OFF; PRAGMA foreign_keys=ON`)
		var enabled int
		if err == nil {
			err = conn.QueryRowContext(cleanup, `PRAGMA foreign_keys`).Scan(&enabled)
		}
		if err != nil || enabled != 1 {
			result = errors.Join(result, errors.New("restore identity migration foreign keys"))
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	if _, err = conn.ExecContext(ctx, `PRAGMA foreign_keys=OFF; PRAGMA legacy_alter_table=ON`); err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	keys, err := identityMigrationKeys(ctx, tx)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err = rekeyIdentityCiphertexts(ctx, tx, vault, key); err != nil {
			return fmt.Errorf("rekey encrypted member documents: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO member_identity_aliases(old_user_id,user_id,old_proxy_id,proxy_id) VALUES(?,?,?,?)`, key.oldUser, key.user, key.oldProxy, key.proxy); err != nil {
			return err
		}
	}
	if err = rekeyIdentityReferences(ctx, tx); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET remna_user_id=(SELECT CAST(proxy_user_id AS TEXT) FROM xnode_identities WHERE user_id=users.id) WHERE EXISTS(SELECT 1 FROM xnode_identities WHERE user_id=users.id)`); err != nil {
		return err
	}
	if err = removeAssignedUsername(ctx, tx); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET onboarding_state='agreement',recovery_reason='' WHERE onboarding_state IN ('username','membership')`); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	invalid := rows.Next()
	err = rows.Err()
	rows.Close()
	if err != nil || invalid {
		return errors.New("identity migration foreign key validation failed")
	}
	return tx.Commit()
}

func identityMigrationKeys(ctx context.Context, tx *sql.Tx) ([]identityRekey, error) {
	rows, err := tx.QueryContext(ctx, `SELECT u.id,CAST(u.telegram_id AS TEXT),i.proxy_user_id,u.telegram_id FROM users u LEFT JOIN xnode_identities i ON i.user_id=u.id ORDER BY u.telegram_id`)
	if err != nil {
		return nil, err
	}
	var keys []identityRekey
	for rows.Next() {
		var key identityRekey
		var telegram int64
		if err = rows.Scan(&key.oldUser, &key.user, &key.oldProxy, &telegram); err != nil {
			rows.Close()
			return nil, err
		}
		if telegram <= 0 || telegram >= 1<<52 {
			rows.Close()
			return nil, errors.New("invalid Telegram identity")
		}
		if key.oldProxy.Valid {
			key.proxy = sql.NullInt64{Int64: telegram, Valid: true}
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT proxy_user_id FROM xnode_identities WHERE user_id IS NULL ORDER BY proxy_user_id`)
	if err != nil {
		return nil, err
	}
	var ordinal int64
	for rows.Next() {
		var old int64
		if err = rows.Scan(&old); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, identityRekey{oldUser: "service:" + strconv.FormatInt(old, 10), oldProxy: sql.NullInt64{Int64: old, Valid: true}, proxy: sql.NullInt64{Int64: (1 << 52) + ordinal, Valid: true}})
		ordinal++
	}
	err = rows.Err()
	rows.Close()
	return keys, err
}
