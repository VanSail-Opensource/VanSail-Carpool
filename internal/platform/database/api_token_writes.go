package database

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func (s *Store) retryAttempt_CreateAdminAPIToken(ctx context.Context, item apitoken.Token) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := catalogAdminActorTx(ctx, tx, item.UserID); err != nil {
		if errors.Is(err, ErrConflict) {
			return apitoken.ErrUnauthorized
		}
		return err
	}
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_api_tokens WHERE user_id=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>$2)`, item.UserID, stamp(item.CreatedAt)).Scan(&active); err != nil {
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
	if _, err = tx.ExecContext(ctx, `INSERT INTO admin_api_tokens(id,user_id,name,token_prefix,token_hash,scopes_json,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, item.ID, item.UserID, item.Name, item.Prefix, item.Hash, string(scopes), stamp(item.CreatedAt), expires); err != nil {
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

func (s *Store) retryAttempt_RevokeAdminAPIToken(ctx context.Context, id, userID string, now time.Time) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := catalogAdminActorTx(ctx, tx, userID); err != nil {
		if errors.Is(err, ErrConflict) {
			return apitoken.ErrUnauthorized
		}
		return err
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_api_tokens WHERE id=$1 AND user_id=$2) OR EXISTS(SELECT 1 FROM audit_events WHERE target_type='api_token' AND target_id=$3 AND actor_user_id=$4 AND action='api_token.revoked')`, id, userID, id, userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return apitoken.ErrNotFound
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM admin_api_tokens WHERE id=$1 AND user_id=$2`, id, userID)
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
