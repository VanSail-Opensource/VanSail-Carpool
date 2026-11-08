package database

import (
	"context"
	"database/sql"
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
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE sessions SET expires_at=$1,renewed_at=$2 WHERE token_hash=$3 AND scope='browser' AND expires_at>$4 AND renewed_at<=$5`, stamp(expires), stamp(now), hash, stamp(now), stamp(now.Add(-24*time.Hour)))
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
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=$1 AND scope='browser'`, hash)
	return err
}
