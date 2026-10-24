package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) EnqueueDIYReconcile(ctx context.Context, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT user_id FROM xnode_member_diy WHERE user_id>? ORDER BY user_id LIMIT 128`, s.diyCleanupCursor)
	if err != nil {
		return err
	}
	var users []string
	for rows.Next() {
		var user string
		if err = rows.Scan(&user); err != nil {
			rows.Close()
			return err
		}
		users = append(users, user)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, user := range users {
		raw, _ := json.Marshal(map[string]string{"userId": user})
		if err = insertOutboxTx(ctx, tx, xnode.DIYReconcileJob, string(raw), now, now); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if len(users) < 128 {
		s.diyCleanupCursor = ""
	} else {
		s.diyCleanupCursor = users[len(users)-1]
	}
	return nil
}

func (s *Store) retryAttempt_SaveSystemDIY(ctx context.Context, doc xnode.DIYDocument) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE xnode_member_diy SET revision=revision+1,cipher=? WHERE user_id=? AND revision=? AND revision<9007199254740991`, doc.Cipher, doc.UserID, doc.Revision)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return xnode.ErrConflict
	}
	if doc.NativeChanged {
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1`); err != nil {
			return err
		}
	}
	return tx.Commit()
}
