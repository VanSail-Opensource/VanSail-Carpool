package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type pmContentCipher interface {
	Encrypt(string, string) (string, error)
	Decrypt(string, string) (string, error)
}

// ConfigurePMContentVault installs the application vault before any PM workers start.
func (s *Store) ConfigurePMContentVault(vault pmContentCipher) {
	s.pmContentMu.Lock()
	defer s.pmContentMu.Unlock()
	s.pmContentCipher = vault
}

func (s *Store) savePMContentTx(ctx context.Context, tx *sql.Tx, operation string, input model.PMRelayInput, now time.Time) error {
	if len(input.Content) == 0 {
		return nil
	}
	if input.Inbound || len(input.Content) > 128*1024 || !json.Valid(input.Content) {
		return model.ErrPMContentInvalid
	}
	vault := s.pmVault()
	if vault == nil {
		return model.ErrPMContentInvalid
	}
	encrypted, err := vault.Encrypt("telegram_pm:"+operation, string(input.Content))
	if err != nil {
		return model.ErrPMContentInvalid
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO telegram_pm_payloads(operation_id,encrypted_payload,expires_at) VALUES($1,$2,$3)`, operation, encrypted, now.Add(24*time.Hour).Unix())
	return err
}

// PMRelayContent decrypts only a live operation-bound pending envelope.
func (s *Store) PMRelayContent(ctx context.Context, operation string, now time.Time) (json.RawMessage, bool, error) {
	var encrypted string
	var expires int64
	err := s.db.QueryRowContext(ctx, `SELECT encrypted_payload,expires_at FROM telegram_pm_payloads WHERE operation_id=$1`, operation).Scan(&encrypted, &expires)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if expires <= now.Unix() || encrypted == "" {
		return nil, true, model.ErrPMContentExpired
	}
	vault := s.pmVault()
	if vault == nil {
		return nil, true, model.ErrPMContentInvalid
	}
	plain, err := vault.Decrypt("telegram_pm:"+operation, encrypted)
	if err != nil || !json.Valid([]byte(plain)) {
		return nil, true, model.ErrPMContentInvalid
	}
	return json.RawMessage(plain), true, nil
}

func (s *Store) pmVault() pmContentCipher {
	s.pmContentMu.RLock()
	defer s.pmContentMu.RUnlock()
	return s.pmContentCipher
}
