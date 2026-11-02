package database

import (
	"database/sql"
	"encoding/json"

	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func identityRekeyFixture(t *testing.T, db *sql.DB) (*Store, *secret.Vault, time.Time) {
	t.Helper()
	at := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO users(id,telegram_id,telegram_first_name,username,onboarding_state,created_at,updated_at) VALUES('legacy-owner',99101,'Owner','reserved','complete',$1,$2)`, stamp(at), stamp(at)); err != nil {
		t.Fatal(err)
	}
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	return NewStore(db), vault, at
}

func operationJobCipher(t *testing.T, db *sql.DB, operation string) string {
	t.Helper()
	var payload string
	if err := db.QueryRow(`SELECT payload FROM outbox_jobs WHERE kind='provider_operation' AND (CASE ((payload)::jsonb #> '{operationId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{operationId}' END)=$1`, operation).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &values); err != nil {
		t.Fatal(err)
	}
	var cipher string
	if err := json.Unmarshal(values["sealedTarget"], &cipher); err != nil {
		t.Fatal(err)
	}
	return cipher
}
