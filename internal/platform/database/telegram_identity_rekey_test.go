package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestTelegramIdentityMigrationRekeysMemberOperationTargets(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "identity-rekey.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, vault, at := identityRekeyFixture(t, db)
	const fingerprint = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	targets := []struct {
		kind, key, before, after string
	}{
		{"xnode_command", "command", "xnode.command.legacy-owner.command", "xnode.command.99101.command"},
		{"xnode_migration", "migration", "xnode.migration.operation.legacy-owner.migration", "xnode.migration.operation.99101.migration"},
		{"xnode_route_speedtest", "speedtest", "xnode.route-speedtest.legacy-owner.speedtest", "xnode.route-speedtest.99101.speedtest"},
		{"emby_preferences", "preferences", "emby-operation:legacy-owner:emby_preferences:" + fingerprint, "emby-operation:99101:emby_preferences:" + fingerprint},
		{"emby_password", "password", "emby-operation:legacy-owner:emby_password:" + fingerprint, "emby-operation:99101:emby_password:" + fingerprint},
		{"connection_drop", "drop", "connection-drop:legacy-owner:" + fingerprint, "connection-drop:99101:" + fingerprint},
	}
	operations := make([]string, 0, len(targets))
	for index, target := range targets {
		sealed, err := vault.Encrypt(target.before, fmt.Sprintf("target-%d", index))
		if err != nil {
			t.Fatal(err)
		}
		operation, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{
			ActorUserID: "legacy-owner", OwnerUserID: "legacy-owner", Kind: target.kind,
			IdempotencyKey: target.key, RequestFingerprint: fingerprint, SealedTarget: sealed,
			Items: []providerops.ItemInput{{Key: "target", TargetType: "test", TargetID: target.key}},
		}, at)
		if err != nil {
			t.Fatal(err)
		}
		operations = append(operations, operation.Receipt.ID)
	}
	blockDigest := strings.Repeat("b", 64)
	blockCipher, err := vault.Encrypt("connection-ip-block:legacy-owner:node-a:"+blockDigest, "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO connection_ip_blocks(id,user_id,node_uuid,ip_digest,sealed_ip,status,expiry_job_id,created_at,updated_at,expires_at) VALUES('block-a','legacy-owner','node-a',? ,?,'active','expiry-a',?,?,?)`, blockDigest, blockCipher, stamp(at), stamp(at), stamp(at)); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateTelegramIdentities(ctx, vault); err != nil {
		t.Fatal(err)
	}
	for index, operation := range operations {
		cipher := operationJobCipher(t, db, operation)
		plain, err := vault.Decrypt(targets[index].after, cipher)
		if err != nil || plain != fmt.Sprintf("target-%d", index) {
			t.Fatalf("operation %s target: %q %v", targets[index].kind, plain, err)
		}
	}
	var cipher string
	if err = db.QueryRow(`SELECT sealed_ip FROM connection_ip_blocks WHERE id='block-a' AND user_id='99101'`).Scan(&cipher); err != nil {
		t.Fatal(err)
	}
	plain, err := vault.Decrypt("connection-ip-block:99101:node-a:"+blockDigest, cipher)
	if err != nil || plain != "203.0.113.8" {
		t.Fatalf("connection block: %q %v", plain, err)
	}
}

func TestTelegramIdentityMigrationAllocatesServiceIDsBeyondExistingMaximum(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "identity-services.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, vault, at := identityRekeyFixture(t, db)
	existing := serviceProxyIDFloor + 17
	if _, err = db.Exec(`INSERT INTO xnode_identities(proxy_user_id,service_name,credentials_cipher,created_at) VALUES(?,'observer','unused',?)`, existing, stamp(at)); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateTelegramIdentities(ctx, vault); err != nil {
		t.Fatal(err)
	}
	var allocated int64
	if err = db.QueryRow(`SELECT proxy_user_id FROM xnode_identities WHERE service_name='observer'`).Scan(&allocated); err != nil {
		t.Fatal(err)
	}
	if allocated != existing+1 {
		t.Fatalf("service proxy ID = %d, want %d", allocated, existing+1)
	}
}

func identityRekeyFixture(t *testing.T, db *sql.DB) (*Store, *secret.Vault, time.Time) {
	t.Helper()
	at := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO users(id,telegram_id,telegram_first_name,username,onboarding_state,created_at,updated_at) VALUES('legacy-owner',99101,'Owner','reserved','complete',?,?)`, stamp(at), stamp(at)); err != nil {
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
	if err := db.QueryRow(`SELECT payload FROM outbox_jobs WHERE kind='provider_operation' AND json_extract(payload,'$.operationId')=?`, operation).Scan(&payload); err != nil {
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
