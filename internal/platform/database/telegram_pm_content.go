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
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.pmContentCipher = vault
}

func (s *Store) savePMContentTx(ctx context.Context, tx *sql.Tx, operation string, input model.PMRelayInput, now time.Time) error {
	if len(input.Content) == 0 {
		return nil
	}
	if input.Inbound || len(input.Content) > 128*1024 || !json.Valid(input.Content) || s.pmContentCipher == nil {
		return model.ErrPMContentInvalid
	}
	encrypted, err := s.pmContentCipher.Encrypt("telegram_pm:"+operation, string(input.Content))
	if err != nil {
		return model.ErrPMContentInvalid
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO telegram_pm_payloads(operation_id,encrypted_payload,expires_at) VALUES(?,?,?)`, operation, encrypted, now.Add(24*time.Hour).Unix())
	return err
}

// PMRelayContent decrypts only a live operation-bound pending envelope.
func (s *Store) PMRelayContent(ctx context.Context, operation string, now time.Time) (json.RawMessage, bool, error) {
	var encrypted string
	var expires int64
	err := s.db.QueryRowContext(ctx, `SELECT encrypted_payload,expires_at FROM telegram_pm_payloads WHERE operation_id=?`, operation).Scan(&encrypted, &expires)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if expires <= now.Unix() || encrypted == "" {
		return nil, true, model.ErrPMContentExpired
	}
	s.writeMu.Lock()
	vault := s.pmContentCipher
	s.writeMu.Unlock()
	if vault == nil {
		return nil, true, model.ErrPMContentInvalid
	}
	plain, err := vault.Decrypt("telegram_pm:"+operation, encrypted)
	if err != nil || !json.Valid([]byte(plain)) {
		return nil, true, model.ErrPMContentInvalid
	}
	return json.RawMessage(plain), true, nil
}
