package postgres

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type recoveryPaymentProfile struct {
	id, provider, cipher, legacyValue string
	legacyEncrypted                   int64
}

func recoveryPaymentLegacyKey(id, provider string) string {
	if id != provider {
		return ""
	}
	switch provider {
	case "ezpay":
		return "billing.ezpay.key"
	case "bepusdt":
		return "billing.bepusdt.api_token"
	default:
		return ""
	}
}

func verifyRecoveryPaymentProfile(vault *secret.Vault, p recoveryPaymentProfile) error {
	if _, err := vault.Decrypt("payment-profile:"+p.id, p.cipher); err == nil {
		return nil
	}
	// Historical migrations copied the original encrypted setting into these
	// two built-in rows without resealing. Never extend that proof to custom
	// IDs or replace a mismatched profile with an unrelated current secret.
	key := recoveryPaymentLegacyKey(p.id, p.provider)
	if key != "" && p.legacyEncrypted == 1 && p.cipher != "" && p.cipher == p.legacyValue {
		if _, err := vault.Decrypt(key, p.cipher); err == nil {
			return nil
		}
	}
	return errors.New("encrypted document verification failed")
}

func verifyRecoveryPaymentProfiles(ctx context.Context, q queryer, vault *secret.Vault) error {
	// One join avoids querying a second result set on an already busy snapshot
	// connection; retained setting equality and its encrypted flag are mandatory.
	rows, err := q.QueryContext(ctx, `SELECT p.id,p.provider,p.credential_ciphertext,COALESCE(s.value,''),COALESCE(s.encrypted,0)
 FROM payment_profiles p LEFT JOIN settings s ON s.key=CASE
 WHEN p.id='ezpay' AND p.provider='ezpay' THEN 'billing.ezpay.key'
 WHEN p.id='bepusdt' AND p.provider='bepusdt' THEN 'billing.bepusdt.api_token' END
 WHERE p.credential_ciphertext<>'' ORDER BY p.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var p recoveryPaymentProfile
		if err = rows.Scan(&p.id, &p.provider, &p.cipher, &p.legacyValue, &p.legacyEncrypted); err != nil {
			return err
		}
		if err = verifyRecoveryPaymentProfile(vault, p); err != nil {
			return err
		}
	}
	return rows.Err()
}
