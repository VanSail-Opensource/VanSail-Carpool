package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func (s *Store) CreateAdminAPIToken(ctx context.Context, item apitoken.Token) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_api_tokens WHERE user_id=? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>?)`, item.UserID, stamp(item.CreatedAt)).Scan(&active); err != nil {
		return err
	}
	if active >= 50 {
		return apitoken.ErrLimit
	}
	scopes, err := json.Marshal(item.Scopes)
	if err != nil {
		return err
	}
	var expires any
	if item.ExpiresAt != nil {
		expires = stamp(*item.ExpiresAt)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO admin_api_tokens(id,user_id,name,token_prefix,token_hash,scopes_json,created_at,expires_at) VALUES(?,?,?,?,?,?,?,?)`, item.ID, item.UserID, item.Name, item.Prefix, item.Hash, string(scopes), stamp(item.CreatedAt), expires); err != nil {
		return err
	}
	auditID, err := ids.New()
	if err != nil {
		return err
	}
	detail, err := json.Marshal(map[string]any{"name": item.Name, "scopes": item.Scopes, "expiresAt": item.ExpiresAt})
	if err != nil {
		return err
	}
	if err = insertAuditTx(ctx, tx, auditID, &item.UserID, "api_token.created", "api_token", item.ID, string(detail), item.CreatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RevokeAdminAPIToken(ctx context.Context, id, userID string, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_api_tokens WHERE id=? AND user_id=?) OR EXISTS(SELECT 1 FROM audit_events WHERE target_type='api_token' AND target_id=? AND actor_user_id=? AND action='api_token.revoked')`, id, userID, id, userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return apitoken.ErrNotFound
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM admin_api_tokens WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count > 0 {
		auditID, err := ids.New()
		if err != nil {
			return err
		}
		if err = insertAuditTx(ctx, tx, auditID, &userID, "api_token.revoked", "api_token", id, "{}", now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
