package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
)

const apiTokenSelect = `SELECT id,user_id,name,token_prefix,token_hash,scopes_json,created_at,expires_at,last_used_at,revoked_at FROM admin_api_tokens`

func scanAPIToken(row rowScanner) (apitoken.Token, error) {
	var result apitoken.Token
	var scopes, created string
	var expires, used, revoked sql.NullString
	err := row.Scan(&result.ID, &result.UserID, &result.Name, &result.Prefix, &result.Hash, &scopes, &created, &expires, &used, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return result, apitoken.ErrNotFound
	}
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal([]byte(scopes), &result.Scopes); err != nil {
		return result, err
	}
	if result.CreatedAt, err = parseStamp(created); err != nil {
		return result, err
	}
	if result.ExpiresAt, err = parseOptionalStamp(expires); err != nil {
		return result, err
	}
	if result.LastUsedAt, err = parseOptionalStamp(used); err != nil {
		return result, err
	}
	result.RevokedAt, err = parseOptionalStamp(revoked)
	return result, err
}

func (s *Store) AdminAPITokens(ctx context.Context, userID string) ([]apitoken.Token, error) {
	// All active keys fit in the first page (creation caps them at 50).
	// Revocations delete credentials; only secret-free audit events survive.
	rows, err := s.db.QueryContext(ctx, apiTokenSelect+` WHERE user_id=? ORDER BY (revoked_at IS NULL AND (expires_at IS NULL OR expires_at>?)) DESC,created_at DESC,id DESC LIMIT 200`, userID, stamp(time.Now().UTC()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []apitoken.Token{}
	for rows.Next() {
		item, err := scanAPIToken(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AdminAPITokenByHash(ctx context.Context, hash []byte) (apitoken.Token, error) {
	item, err := scanAPIToken(s.db.QueryRowContext(ctx, apiTokenSelect+` WHERE token_hash=?`, hash))
	if errors.Is(err, apitoken.ErrNotFound) {
		return item, apitoken.ErrUnauthorized
	}
	return item, err
}

func (s *Store) TouchAdminAPIToken(ctx context.Context, id string, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE admin_api_tokens SET last_used_at=? WHERE id=? AND revoked_at IS NULL AND (last_used_at IS NULL OR last_used_at<=?)`, stamp(now), id, stamp(now.Add(-5*time.Minute)))
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		var active bool
		if err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_api_tokens WHERE id=? AND revoked_at IS NULL)`, id).Scan(&active); err != nil {
			return err
		}
		if !active {
			return apitoken.ErrUnauthorized
		}
	}
	return err
}
