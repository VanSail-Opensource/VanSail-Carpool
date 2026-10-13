package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

// CreateBrowserSession adds an independent browser session without replacing Mini App or other device sessions.
func (s *Store) CreateBrowserSession(ctx context.Context, hash []byte, userID string, now, expires time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,scope,expires_at,created_at,renewed_at) VALUES(?,?,'browser',?,?,?)`, hash, userID, stamp(expires), stamp(now), stamp(now))
	if err != nil {
		return fmt.Errorf("create browser session: %w", err)
	}
	return nil
}

// BrowserSession returns the owner and actual expiry of a live browser-scoped session.
func (s *Store) BrowserSession(ctx context.Context, hash []byte, now time.Time) (model.User, time.Time, error) {
	user, err := scanUser(s.db.QueryRowContext(ctx, userSelect+` JOIN sessions ON sessions.user_id=users.id WHERE sessions.token_hash=? AND sessions.scope='browser' AND sessions.expires_at>?`, hash, stamp(now)))
	if err != nil {
		return model.User{}, time.Time{}, err
	}
	var raw string
	if err = s.db.QueryRowContext(ctx, `SELECT expires_at FROM sessions WHERE token_hash=? AND scope='browser' AND expires_at>?`, hash, stamp(now)).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, time.Time{}, ErrNotFound
		}
		return model.User{}, time.Time{}, err
	}
	expires, err := parseStamp(raw)
	return user, expires, err
}

// RenewBrowserSession extends only a live session, at most daily; the bearer and signing key remain stable.
func (s *Store) RenewBrowserSession(ctx context.Context, hash []byte, now, expires time.Time) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE sessions SET expires_at=?,renewed_at=? WHERE token_hash=? AND scope='browser' AND expires_at>? AND renewed_at<=?`, stamp(expires), stamp(now), hash, stamp(now), stamp(now.Add(-24*time.Hour)))
	if err != nil {
		return false, fmt.Errorf("renew browser session: %w", err)
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

// DeleteBrowserSession revokes one browser device without affecting any other session.
func (s *Store) DeleteBrowserSession(ctx context.Context, hash []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=? AND scope='browser'`, hash)
	return err
}
