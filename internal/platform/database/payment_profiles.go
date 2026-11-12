package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PaymentProfileInput contains one provider-account profile and its enabled channels.
type PaymentProfileInput struct {
	ActorID                                          string
	ID, Provider, ProviderName, Endpoint, MerchantID string
	EnabledChannels                                  []string
	CredentialCiphertext, Acknowledgement            string
	Enabled                                          bool
}

// PaymentProfileRecord is the private storage representation of a provider.
type PaymentProfileRecord struct {
	model.PaymentProfile
	CredentialCiphertext string
}

func (s *Store) SavePaymentProfile(ctx context.Context, input PaymentProfileInput) (model.PaymentProfile, error) {
	for attempt := 0; ; attempt++ {
		profile, err := s.savePaymentProfile(ctx, input)
		if !s.retryTransaction(ctx, err, attempt) {
			return profile, err
		}
	}
}

func (s *Store) savePaymentProfile(ctx context.Context, input PaymentProfileInput) (model.PaymentProfile, error) {
	if input.ID == "" || input.Provider == "" || input.Endpoint == "" {
		return model.PaymentProfile{}, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PaymentProfile{}, fmt.Errorf("begin payment profile update: %w", err)
	}
	defer tx.Rollback()
	if err := paymentProfileActorTx(ctx, tx, input.ActorID); err != nil {
		return model.PaymentProfile{}, err
	}
	var previousProvider, previousEndpoint, previousMerchant, previousCredential string
	err = tx.QueryRowContext(ctx, `SELECT provider,endpoint,merchant_id,credential_ciphertext FROM payment_profiles WHERE id=$1 FOR UPDATE`, input.ID).
		Scan(&previousProvider, &previousEndpoint, &previousMerchant, &previousCredential)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.PaymentProfile{}, fmt.Errorf("lock payment profile: %w", err)
	}
	if err == nil && (input.Provider != previousProvider || input.Endpoint != previousEndpoint || input.MerchantID != previousMerchant ||
		input.CredentialCiphertext != "" && input.CredentialCiphertext != previousCredential) {
		if referenced, err := paymentProfileReferencedTx(ctx, tx, input.ID, previousProvider); err != nil {
			return model.PaymentProfile{}, err
		} else if referenced {
			return model.PaymentProfile{}, ErrPaymentProfileInUse
		}
	}
	providerName := strings.TrimSpace(input.ProviderName)
	if providerName == "" {
		providerName = input.Provider
	}
	now := nowUTC()
	result, err := tx.ExecContext(ctx, `INSERT INTO payment_profiles(id,provider,provider_name,enabled_channels,endpoint,merchant_id,credential_ciphertext,acknowledgement,enabled,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(id) DO UPDATE SET provider=excluded.provider,provider_name=excluded.provider_name,enabled_channels=excluded.enabled_channels,endpoint=excluded.endpoint,merchant_id=excluded.merchant_id,credential_ciphertext=CASE WHEN excluded.credential_ciphertext='' THEN payment_profiles.credential_ciphertext ELSE excluded.credential_ciphertext END,acknowledgement=excluded.acknowledgement,enabled=excluded.enabled,updated_at=excluded.updated_at
		WHERE excluded.credential_ciphertext<>'' OR
			(payment_profiles.provider=excluded.provider AND payment_profiles.endpoint=excluded.endpoint AND payment_profiles.merchant_id=excluded.merchant_id)`,
		input.ID, input.Provider, providerName, strings.Join(input.EnabledChannels, ","), input.Endpoint, input.MerchantID, input.CredentialCiphertext, input.Acknowledgement, boolInt(input.Enabled), stamp(now), stamp(now))
	if err != nil {
		return model.PaymentProfile{}, fmt.Errorf("save payment profile: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return model.PaymentProfile{}, fmt.Errorf("inspect payment profile update: %w", err)
	} else if affected != 1 {
		// An omitted credential means retain it for the same provider account,
		// never forward a write-only secret to a replacement destination.
		return model.PaymentProfile{}, ErrConflict
	}
	if err := paymentProfileAuditTx(ctx, tx, input.ActorID, "payment_profile.update", input.ID, now); err != nil {
		return model.PaymentProfile{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.PaymentProfile{}, fmt.Errorf("commit payment profile update: %w", err)
	}
	record, err := s.PaymentProfileRecordByID(ctx, input.ID, "")
	if err != nil {
		return model.PaymentProfile{}, err
	}
	return record.PaymentProfile, nil
}

func (s *Store) ListPaymentProfiles(ctx context.Context) ([]model.PaymentProfile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,provider,provider_name,enabled_channels,endpoint,merchant_id,credential_ciphertext,acknowledgement,enabled FROM payment_profiles ORDER BY provider,provider_name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.PaymentProfile, 0)
	for rows.Next() {
		profile, err := scanPaymentProfile(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, profile.PaymentProfile)
	}
	return result, rows.Err()
}

// SetPaymentProfileEnabled changes availability without rewriting credentials.
func (s *Store) SetPaymentProfileEnabled(ctx context.Context, id string, enabled bool) (model.PaymentProfile, error) {
	for attempt := 0; ; attempt++ {
		profile, err := s.setPaymentProfileEnabled(ctx, id, enabled)
		if !s.retryTransaction(ctx, err, attempt) {
			return profile, err
		}
	}
}

func (s *Store) setPaymentProfileEnabled(ctx context.Context, id string, enabled bool) (model.PaymentProfile, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PaymentProfile{}, err
	}
	defer tx.Rollback()
	var previous int
	if err := tx.QueryRowContext(ctx, `SELECT enabled FROM payment_profiles WHERE id=$1 FOR UPDATE`, id).Scan(&previous); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.PaymentProfile{}, ErrNotFound
		}
		return model.PaymentProfile{}, err
	}
	now := nowUTC()
	result, err := tx.ExecContext(ctx, `UPDATE payment_profiles SET enabled=$1,updated_at=$2 WHERE id=$3`,
		boolInt(enabled), stamp(now), id)
	if err != nil {
		return model.PaymentProfile{}, fmt.Errorf("set payment profile enabled: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return model.PaymentProfile{}, rowsErr
		}
		return model.PaymentProfile{}, ErrNotFound
	}
	if previous != boolInt(enabled) {
		if err := paymentProfileAuditTx(ctx, tx, "", "payment_profile.availability", id, now); err != nil {
			return model.PaymentProfile{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.PaymentProfile{}, err
	}
	record, err := s.PaymentProfileRecordByID(ctx, id, "")
	return record.PaymentProfile, err
}

// DeletePaymentProfile removes a profile only after its provider attempts are gone.
func (s *Store) retryAttempt_DeletePaymentProfile(ctx context.Context, id, actorID string) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin payment profile deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := paymentProfileActorTx(ctx, tx, actorID); err != nil {
		return err
	}
	var provider string
	if err := tx.QueryRowContext(ctx, `SELECT provider FROM payment_profiles WHERE id=$1 FOR UPDATE`, id).Scan(&provider); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load payment profile for deletion: %w", err)
	}
	if referenced, err := paymentProfileReferencedTx(ctx, tx, id, provider); err != nil {
		return err
	} else if referenced {
		return ErrPaymentProfileInUse
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM payment_profiles WHERE id=$1`, id); err != nil {
		return fmt.Errorf("delete payment profile: %w", err)
	}
	if err := paymentProfileAuditTx(ctx, tx, actorID, "payment_profile.delete", id, nowUTC()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit payment profile deletion: %w", err)
	}
	return nil
}

func (s *Store) PaymentProfile(ctx context.Context, provider, rail string) (model.PaymentProfile, error) {
	record, err := s.paymentProfileRecord(ctx, provider, rail)
	return record.PaymentProfile, err
}

func (s *Store) PaymentProfileRecord(ctx context.Context, provider, rail string) (PaymentProfileRecord, error) {
	return s.paymentProfileRecord(ctx, provider, rail)
}

// PaymentProfileRecordByID loads one profile account and optionally checks its rail.
func (s *Store) PaymentProfileRecordByID(ctx context.Context, id, rail string) (PaymentProfileRecord, error) {
	record, err := scanPaymentProfile(s.db.QueryRowContext(ctx, `SELECT id,provider,provider_name,enabled_channels,endpoint,merchant_id,credential_ciphertext,acknowledgement,enabled FROM payment_profiles WHERE id=$1`, id))
	record.Rail = rail
	return record, err
}

func (s *Store) paymentProfileRecord(ctx context.Context, provider, rail string) (PaymentProfileRecord, error) {
	query := `SELECT id,provider,provider_name,enabled_channels,endpoint,merchant_id,credential_ciphertext,acknowledgement,enabled FROM payment_profiles WHERE provider=$1`
	args := []any{provider}
	if rail != "" {
		query += fmt.Sprintf(" AND (',' || enabled_channels || ',') ILIKE $%d", len(args)+1)

		args = append(args, `%,`+rail+`,%`)
	}
	query += ` ORDER BY id LIMIT 1`
	record, err := scanPaymentProfile(s.db.QueryRowContext(ctx, query, args...))
	record.Rail = rail
	return record, err
}

func scanPaymentProfile(row interface{ Scan(...any) error }) (PaymentProfileRecord, error) {
	var record PaymentProfileRecord
	var channels string
	var enabled int
	if err := row.Scan(&record.ID, &record.Provider, &record.ProviderName, &channels, &record.Endpoint, &record.MerchantID, &record.CredentialCiphertext, &record.Acknowledgement, &enabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PaymentProfileRecord{}, ErrNotFound
		}
		return PaymentProfileRecord{}, fmt.Errorf("scan payment profile: %w", err)
	}
	record.Enabled = enabled == 1
	for _, channel := range strings.Split(channels, ",") {
		channel = strings.TrimSpace(channel)
		if channel != "" {
			record.EnabledChannels = append(record.EnabledChannels, channel)
		}
	}
	if record.ProviderName == "" {
		record.ProviderName = record.Provider
	}
	record.Configured = record.Endpoint != "" && record.CredentialCiphertext != ""
	if record.Configured {
		record.Credential = "********"
	}
	return record, nil
}

func nowUTC() time.Time { return time.Now().UTC() }
