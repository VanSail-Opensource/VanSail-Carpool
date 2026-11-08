package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
)

// HourlyPolicySettingKey is reserved for the typed encrypted policy interface.
const HourlyPolicySettingKey = "xnode.hourly_policy"
const maxHourlyPolicyCipherBytes = 16 << 10

// HourlyPolicyRecord contains encrypted settings and their opaque CAS version.
// The database never decrypts policy documents or fingerprints plaintext.
type HourlyPolicyRecord struct {
	Ciphertext string
	Version    string
}

type hourlySettingsReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func hourlyPolicyRecord(ctx context.Context, reader hourlySettingsReader, lock string) (HourlyPolicyRecord, error) {
	var record HourlyPolicyRecord
	var encrypted int
	err := reader.QueryRowContext(ctx, `SELECT value,encrypted FROM settings WHERE key=$1`+lock, HourlyPolicySettingKey).Scan(&record.Ciphertext, &encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return record, nil // Unconfigured means sales disabled, with no version.
	}
	if err != nil {
		return HourlyPolicyRecord{}, err
	}
	if encrypted != 1 || record.Ciphertext == "" || len(record.Ciphertext) > maxHourlyPolicyCipherBytes {
		return HourlyPolicyRecord{}, ErrConflict
	}
	record.Version = hourlyPolicyFingerprint(record.Ciphertext)
	return record, nil
}

func hourlyPolicyFingerprint(ciphertext string) string {
	digest := sha256.Sum256([]byte(ciphertext))
	return hex.EncodeToString(digest[:])
}

// ReadHourlyPolicyRecord returns one exact encrypted snapshot, or an absent row.
func (s *Store) ReadHourlyPolicyRecord(ctx context.Context) (HourlyPolicyRecord, error) {
	return hourlyPolicyRecord(ctx, s.db, "")
}

// lockHourlyPolicyVersionTx fences a validated service snapshot before debit.
// Callers compare its version while this shared lock remains held. An absent
// row cannot authorize a purchase, so first-time setup needs no phantom lock.
func lockHourlyPolicyVersionTx(ctx context.Context, tx *sql.Tx) (string, error) {
	record, err := hourlyPolicyRecord(ctx, tx, " FOR SHARE")
	return record.Version, err
}
