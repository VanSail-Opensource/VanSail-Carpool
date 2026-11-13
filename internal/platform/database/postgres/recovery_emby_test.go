package postgres_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
)

func TestRecoveryAuthenticatesRetainedEmbyCommandTargets(t *testing.T) {
	t.Parallel()
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
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = db.ExecContext(ctx, `INSERT INTO users(id,telegram_id,created_at,updated_at) VALUES('recovery-emby-owner',606061,$1,$1)`, stamp); err != nil {
		t.Fatal(err)
	}
	kinds := []string{"emby_setup", "emby_preferences", "emby_password", "emby_provision_retry", "payment_refund"}
	fingerprint := strings.Repeat("a", 64)
	payloads := make(map[string]string)
	encode := func(kind, target string) string {
		t.Helper()
		raw, err := json.Marshal(map[string]string{"operationId": kind, "sealedTarget": target})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	for _, kind := range kinds {
		target := `{"orderId":"synthetic-order","reason":"provider refund"}`
		if kind != "payment_refund" {
			target, err = vault.Encrypt("emby-operation:recovery-emby-owner:"+kind+":"+fingerprint, `{"password":"synthetic-private-target"}`)
			if err != nil {
				t.Fatal(err)
			}
		}
		payloads[kind] = encode(kind, target)
		if _, err = db.ExecContext(ctx, `INSERT INTO provider_operations(id,actor_user_id,owner_user_id,kind,idempotency_key,request_fingerprint,status,created_at,updated_at)
 VALUES($1,'recovery-emby-owner','recovery-emby-owner',$1,$1,$2,'queued',$3,$3)`, kind, fingerprint, stamp); err != nil {
			t.Fatal(err)
		}
		if _, err = db.ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,payload,status,available_at,created_at,updated_at)
 VALUES($1,'provider_operation',$2,'pending',$3,$3,$3)`, kind, payloads[kind], stamp); err != nil {
			t.Fatal(err)
		}
	}
	verify := func(wantError bool) {
		t.Helper()
		tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
		if err != nil {
			t.Fatal(err)
		}
		err = postgres.VerifyEncryption(ctx, tx, key)
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			t.Fatal(rollbackErr)
		}
		if (err != nil) != wantError {
			t.Fatal("unexpected encrypted-command recovery classification", err)
		}
		if err != nil && err.Error() != "encrypted document verification failed" {
			t.Fatal("recovery exposed command details", err)
		}
	}
	verify(false) // The unrelated refund target is deliberately plaintext JSON.
	for _, kind := range kinds[:4] {
		wrongOwner, err := vault.Encrypt("emby-operation:another-owner:"+kind+":"+fingerprint, "synthetic-private-target")
		if err != nil {
			t.Fatal(err)
		}
		tampered := encode(kind, wrongOwner)
		if _, err = db.ExecContext(ctx, `UPDATE outbox_jobs SET payload=$1 WHERE id=$2`, tampered, kind); err != nil {
			t.Fatal(err)
		}
		verify(true)
		var retained string
		if err := db.QueryRowContext(ctx, `SELECT payload FROM outbox_jobs WHERE id=$1`, kind).Scan(&retained); err != nil || retained != tampered {
			t.Fatal("recovery verification rewrote its input evidence", err)
		}
		if _, err = db.ExecContext(ctx, `UPDATE outbox_jobs SET payload=$1 WHERE id=$2`, payloads[kind], kind); err != nil {
			t.Fatal(err)
		}
	}
	verify(false)
}
