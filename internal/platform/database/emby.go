package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
)

const embyAccountSelect = `SELECT id,user_id,base_username,remote_user_id,remote_username,candidate_username,status,
	setup_price_txb_minor,setup_attempt,password_ciphertext,password_context,max_parental_rating,create_attempted,
	last_error,created_at,updated_at,provisioned_at,refunded_at,pending_preferences_json FROM emby_accounts`

// EmbyBaseUsername returns the user's immutable local Remnawave username.
func (s *Store) EmbyBaseUsername(ctx context.Context, userID string) (string, error) {
	var username string
	err := s.db.QueryRowContext(ctx, `SELECT username FROM users WHERE id=? AND username IS NOT NULL`, userID).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("load Emby base username: %w", err)
	}
	return username, nil
}

// EmbyAccountForUser returns the safe account view for a local user.
func (s *Store) EmbyAccountForUser(ctx context.Context, userID string) (domain.Account, error) {
	record, err := scanEmbyProvisioning(s.db.QueryRowContext(ctx, embyAccountSelect+` WHERE user_id=?`, userID))
	if err != nil {
		return domain.Account{}, err
	}
	err = hydrateEmbyPreferences(ctx, s.db, &record)
	return record.Account, err
}

// ListEmbyAccounts returns safe account states without provisioning secrets.
func (s *Store) ListEmbyAccounts(ctx context.Context, limit int) ([]domain.Account, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, embyAccountSelect+` ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list Emby accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	accounts := make([]domain.Account, 0)
	for rows.Next() {
		record, scanErr := scanEmbyProvisioning(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		scanErr = hydrateEmbyPreferences(ctx, s.db, &record)
		if scanErr != nil {
			return nil, scanErr
		}
		accounts = append(accounts, record.Account)
	}
	return accounts, rows.Err()
}

// EmbyProvisioningByID returns one server-only durable provisioning record.
func (s *Store) EmbyProvisioningByID(ctx context.Context, accountID string) (domain.ProvisioningRecord, error) {
	record, err := scanEmbyProvisioning(s.db.QueryRowContext(ctx, embyAccountSelect+` WHERE id=?`, accountID))
	if err != nil {
		return domain.ProvisioningRecord{}, err
	}
	err = hydrateEmbyPreferences(ctx, s.db, &record)
	return record, err
}
