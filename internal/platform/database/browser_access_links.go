package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CreateBrowserAccessLink stores a short-lived, hashed browser handoff reference.
func (s *Store) CreateBrowserAccessLink(ctx context.Context, hash []byte, userID, returnTo string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO browser_access_links(token_hash,user_id,return_to,expires_at) VALUES($1,$2,$3,$4)`, hash, userID, returnTo, stamp(expires))
	return err
}

// ExchangeBrowserAccessLink authorizes the locked owner and destination before
// atomically consuming the handoff and creating its browser session. authorize
// must be a pure local predicate: transaction retries may call it again.
func (s *Store) retryAttempt_ExchangeBrowserAccessLink(ctx context.Context, linkHash, sessionHash []byte, now, expires time.Time, authorize func(model.User, string) bool) (model.User, string, error) {
	if authorize == nil || !expires.After(now) {
		return model.User{}, "", ErrNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.User{}, "", err
	}
	defer func() { _ = tx.Rollback() }()
	var userID, returnTo, rawExpiry string
	err = tx.QueryRowContext(ctx, `DELETE FROM browser_access_links WHERE token_hash=$1 RETURNING user_id,return_to,expires_at`, linkHash).Scan(&userID, &returnTo, &rawExpiry)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, "", ErrNotFound
	}
	if err != nil {
		return model.User{}, "", fmt.Errorf("consume browser link: %w", err)
	}
	linkExpiry, err := parseStamp(rawExpiry)
	if err != nil {
		return model.User{}, "", err
	}
	if !linkExpiry.After(now) {
		return model.User{}, "", ErrNotFound
	}
	user, err := scanUser(tx.QueryRowContext(ctx, userSelect+` WHERE id=$1 FOR UPDATE`, userID))
	if err != nil {
		return model.User{}, "", err
	}
	if user.AccountStatus != "active" || user.BotBlocked || !authorize(user, returnTo) {
		return model.User{}, "", ErrNotFound
	}
	if err = cancelCleanupCandidateTx(ctx, tx, user.ID, "authenticated", now); err != nil {
		return model.User{}, "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,scope,expires_at,created_at,renewed_at) VALUES($1,$2,'browser',$3,$4,$5)`, sessionHash, userID, stamp(expires), stamp(now), stamp(now)); err != nil {
		return model.User{}, "", err
	}
	if err = tx.Commit(); err != nil {
		return model.User{}, "", err
	}
	return user, returnTo, nil
}
