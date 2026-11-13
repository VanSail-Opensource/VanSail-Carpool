package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CreateBrowserSession adds an independent browser session without replacing Mini App or other device sessions.
func (s *Store) CreateBrowserSession(ctx context.Context, hash []byte, userID string, now, expires time.Time) error {
	_, err := hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (bool, error) {
		var active bool
		if e := tx.QueryRowContext(ctx, `SELECT account_status='active' AND bot_blocked=0 FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&active); e != nil {
			return false, e
		}
		if !active {
			return false, ErrAccountUnavailable
		}
		e := cancelCleanupCandidateTx(ctx, tx, userID, "authenticated", now)
		if e != nil {
			return false, e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,scope,expires_at,created_at,renewed_at) VALUES($1,$2,'browser',$3,$4,$5)`, hash, userID, stamp(expires), stamp(now), stamp(now))
		return true, e
	})
	if err != nil {
		return fmt.Errorf("create browser session: %w", err)
	}
	return nil
}

// BrowserSession returns the owner and actual expiry of a live browser-scoped session.
func (s *Store) BrowserSession(ctx context.Context, hash []byte, now time.Time) (model.User, time.Time, error) {
	return s.authenticatedSession(ctx, hash, "browser", now)
}

// RenewBrowserSession extends only a live session, at most daily; the bearer and signing key remain stable.
func (s *Store) RenewBrowserSession(ctx context.Context, hash []byte, now, expires time.Time) (bool, error) {
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (bool, error) {
		var rawExpiry, rawRenewed string
		err := tx.QueryRowContext(ctx, `SELECT expires_at,renewed_at FROM sessions WHERE token_hash=$1 AND scope='browser' FOR UPDATE`, hash).Scan(&rawExpiry, &rawRenewed)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		currentExpiry, err := parseStamp(rawExpiry)
		if err != nil {
			return false, err
		}
		if !currentExpiry.After(now) {
			return false, nil
		}
		if rawRenewed != "" {
			renewed, err := parseStamp(rawRenewed)
			if err != nil {
				return false, err
			}
			if renewed.After(now.Add(-24 * time.Hour)) {
				return false, nil
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE sessions SET expires_at=$1,renewed_at=$2 WHERE token_hash=$3 AND scope='browser'`, stamp(expires), stamp(now), hash)
		if err != nil {
			return false, fmt.Errorf("renew browser session: %w", err)
		}
		return true, nil
	})
}

// DeleteBrowserSession revokes one browser device without affecting any other session.
func (s *Store) DeleteBrowserSession(ctx context.Context, hash []byte) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=$1 AND scope='browser'`, hash)
	return err
}
