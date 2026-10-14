package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"strings"
)

func scanEmbyProvisioning(row rowScanner) (domain.ProvisioningRecord, error) {
	var record domain.ProvisioningRecord
	var remoteID, remoteUsername, candidate sql.NullString
	var parentalRating sql.NullInt64
	var provisioned, refunded sql.NullString
	var created, updated string
	var createAttempted int
	if err := row.Scan(&record.ID, &record.UserID, &record.BaseUsername, &remoteID, &remoteUsername, &candidate, &record.Status,
		&record.SetupPriceTXBMinor, &record.SetupAttempt, &record.PasswordCiphertext, &record.PasswordContext,
		&parentalRating, &createAttempted, &record.LastError, &created, &updated, &provisioned, &refunded, &record.PendingPreferencesJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ProvisioningRecord{}, domain.ErrNotFound
		}
		return domain.ProvisioningRecord{}, fmt.Errorf("scan Emby account: %w", err)
	}
	record.RemoteUserID = remoteID.String
	record.RemoteUsername = remoteUsername.String
	record.CandidateUsername = candidate.String
	record.CreateAttempted = createAttempted == 1
	record.Retryable = (record.Status == domain.StatusQueued || record.Status == domain.StatusPendingReview) && record.PasswordCiphertext != ""
	if parentalRating.Valid {
		if parentalRating.Int64 < -2147483648 || parentalRating.Int64 > 2147483647 {
			return domain.ProvisioningRecord{}, errors.New("invalid stored Emby parental rating")
		}
		converted := int32(parentalRating.Int64)
		record.Preferences.MaxParentalRating = &converted
	}
	var err error
	record.CreatedAt, err = parseStamp(created)
	if err != nil {
		return domain.ProvisioningRecord{}, err
	}
	record.UpdatedAt, err = parseStamp(updated)
	if err != nil {
		return domain.ProvisioningRecord{}, err
	}
	if provisioned.Valid {
		value, parseErr := parseStamp(provisioned.String)
		if parseErr != nil {
			return domain.ProvisioningRecord{}, parseErr
		}
		record.ProvisionedAt = &value
	}
	if refunded.Valid {
		value, parseErr := parseStamp(refunded.String)
		if parseErr != nil {
			return domain.ProvisioningRecord{}, parseErr
		}
		record.RefundedAt = &value
	}
	return record, nil
}

type embyQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func hydrateEmbyPreferences(ctx context.Context, queryer embyQueryer, record *domain.ProvisioningRecord) error {
	disabled, err := embyFolderIDs(ctx, queryer, record.ID)
	if err != nil {
		return err
	}
	record.Preferences.DisabledLibraryIDs = disabled
	if (record.Status == domain.StatusQueued || record.Status == domain.StatusProvisioning) && record.PendingPreferencesJSON != "" && record.PendingPreferencesJSON != "{}" {
		var pending domain.Preferences
		if err := json.Unmarshal([]byte(record.PendingPreferencesJSON), &pending); err != nil {
			return fmt.Errorf("decode pending Emby preferences: %w", err)
		}
		record.Preferences = pending
	}
	return nil
}

func embyFolderIDs(ctx context.Context, queryer embyQueryer, accountID string) ([]string, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT folder_id FROM emby_account_disabled_folders WHERE account_id=? ORDER BY folder_id`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list Emby folders: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]string, 0)
	for rows.Next() {
		var folderID string
		if err := rows.Scan(&folderID); err != nil {
			return nil, err
		}
		result = append(result, folderID)
	}
	return result, rows.Err()
}

func replaceEmbyFoldersTx(ctx context.Context, tx *sql.Tx, accountID string, folderIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM emby_account_disabled_folders WHERE account_id=?`, accountID); err != nil {
		return fmt.Errorf("clear Emby folders: %w", err)
	}
	seen := make(map[string]struct{}, len(folderIDs))
	for _, folderID := range folderIDs {
		folderID = strings.TrimSpace(folderID)
		if folderID == "" {
			continue
		}
		if _, exists := seen[folderID]; exists {
			continue
		}
		seen[folderID] = struct{}{}
		if _, err := tx.ExecContext(ctx, `INSERT INTO emby_account_disabled_folders(account_id,folder_id) VALUES(?,?)`, accountID, folderID); err != nil {
			return fmt.Errorf("insert Emby folder: %w", err)
		}
	}
	return nil
}

func nullableInt32(value *int32) any {
	if value == nil {
		return nil
	}
	return *value
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

func mapEmbyNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func truncateEmbyError(value string) string {
	if len(value) > 500 {
		return value[:500]
	}
	return value
}

var _ domain.Repository = (*Store)(nil)
