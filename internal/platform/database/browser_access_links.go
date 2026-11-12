package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CreateBrowserAccessLink stores a short-lived, hashed browser handoff reference.
func (s *Store) CreateBrowserAccessLink(ctx context.Context, hash []byte, userID, returnTo string, expires time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO browser_access_links(token_hash,user_id,return_to,expires_at) VALUES($1,$2,$3,$4)`, hash, userID, returnTo, stamp(expires))
	return err
}

// ExchangeBrowserAccessLink atomically consumes a live handoff and creates its browser session.
func (s *Store) retryAttempt_ExchangeBrowserAccessLink(ctx context.Context, linkHash, sessionHash []byte, now, expires time.Time) (string, string, error) {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback() }()
	var userID, returnTo, rawExpiry string
	err = tx.QueryRowContext(ctx, `DELETE FROM browser_access_links WHERE token_hash=$1 RETURNING user_id,return_to,expires_at`, linkHash).Scan(&userID, &returnTo, &rawExpiry)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("consume browser link: %w", err)
	}
	linkExpiry, err := parseStamp(rawExpiry)
	if err != nil {
		return "", "", err
	}
	if !linkExpiry.After(now) {
		return "", "", ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,scope,expires_at,created_at,renewed_at) VALUES($1,$2,'browser',$3,$4,$5)`, sessionHash, userID, stamp(expires), stamp(now), stamp(now)); err != nil {
		return "", "", err
	}
	if err = tx.Commit(); err != nil {
		return "", "", err
	}
	return userID, returnTo, nil
}
