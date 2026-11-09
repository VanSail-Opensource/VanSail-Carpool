package postgres_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
)

func TestRecoveryEncryptionAuthenticatesMigratedPaymentWithoutChangingEvidence(t *testing.T) {
	for _, test := range []struct {
		name, provider, id      string
		encrypted               int64
		matching, canonical, ok bool
	}{
		{"ezpay-built-in-disabled", "ezpay", "ezpay", 1, true, false, true},
		{"bepusdt-built-in-disabled", "bepusdt", "bepusdt", 1, true, false, true},
		{"custom-canonical", "ezpay", "custom", 1, false, true, true},
		{"custom-copied-legacy-disabled", "ezpay", "custom", 1, true, false, false},
		{"built-in-unrelated-retained-secret", "ezpay", "ezpay", 1, false, false, false},
		{"built-in-plaintext-flag", "ezpay", "ezpay", 0, true, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			db, err := testdatabase.Open(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
			key := make([]byte, 32)
			vault, err := secret.NewVault(key)
			if err != nil {
				t.Fatal(err)
			}
			legacyKey := "billing." + test.provider + ".api_token"
			if test.provider == "ezpay" {
				legacyKey = "billing.ezpay.key"
			}
			cipher, err := vault.Encrypt(legacyKey, "fixture-private-secret")
			if err != nil {
				t.Fatal(err)
			}
			retained := cipher
			if !test.matching {
				retained, err = vault.Encrypt(legacyKey, "fixture-private-secret")
				if err != nil {
					t.Fatal(err)
				}
			}
			if test.canonical {
				cipher, err = vault.Encrypt("payment-profile:"+test.id, "fixture-private-secret")
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err = db.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at) VALUES($1,$2,$3,$4)
 ON CONFLICT(key) DO UPDATE SET value=excluded.value,encrypted=excluded.encrypted,updated_at=excluded.updated_at`, legacyKey, retained, test.encrypted, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				t.Fatal(err)
			}
			result, err := db.ExecContext(ctx, `INSERT INTO payment_profiles(id,provider,provider_name,endpoint,credential_ciphertext,enabled,created_at,updated_at)
 VALUES($1,$2,'Synthetic','',$3,0,$4,$4) ON CONFLICT(id) DO UPDATE
 SET provider=excluded.provider,credential_ciphertext=excluded.credential_ciphertext,enabled=0`, test.id, test.provider, cipher, time.Now().UTC().Format(time.RFC3339Nano))
			if err != nil {
				t.Fatal(err)
			}
			if affected, err := result.RowsAffected(); err != nil || affected != 1 {
				t.Fatal("fixture did not persist exactly one profile", affected, err)
			}
			snapshot := func() string {
				t.Helper()
				var hash string
				if e := db.QueryRowContext(ctx, `SELECT md5((SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM payment_profiles p)||
 (SELECT jsonb_agg(to_jsonb(s) ORDER BY key)::text FROM settings s))`).Scan(&hash); e != nil {
					t.Fatal(e)
				}
				return hash
			}
			before := snapshot()
			tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
			if err != nil {
				t.Fatal(err)
			}
			err = postgres.VerifyEncryption(ctx, tx, key)
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				t.Fatal(rollbackErr)
			}
			if (err == nil) != test.ok {
				t.Fatal("unexpected complete recovery classification", err)
			}
			if err != nil && (err.Error() != "encrypted document verification failed" || strings.Contains(err.Error(), "fixture-private-secret")) {
				t.Fatal("protected recovery material exposed")
			}
			if snapshot() != before {
				t.Fatal("verification rewrote retained settings/profile evidence")
			}
		})
	}
}
