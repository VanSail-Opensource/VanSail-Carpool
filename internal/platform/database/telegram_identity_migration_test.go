package database

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestTelegramIdentityMigrationPreservesSecretsReferencesAndLateTraffic(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	at := stamp(now)
	for index, item := range []struct {
		old             string
		telegram, proxy int64
	}{{"old-member-a", 99101, 42}, {"old-member-b", 42, 84}} {
		if _, err = db.Exec(`INSERT INTO users(id,telegram_id,telegram_first_name,username,onboarding_state,created_at,updated_at) VALUES(?,?,?,'reserved','complete',?,?)`, item.old, item.telegram, fmt.Sprint("Member ", index), at, at); err != nil {
			t.Fatal(err)
		}
		credentials, e := vault.Encrypt("xnode.identity.user."+item.old, fmt.Sprintf(`{"uuid":"secret-%d"}`, index))
		if e != nil {
			t.Fatal(e)
		}
		token, e := vault.Encrypt("xnode.subscription.user."+item.old, strings.Repeat(fmt.Sprint(index), 43))
		if e != nil {
			t.Fatal(e)
		}
		if _, err = db.Exec(`INSERT INTO xnode_identities(proxy_user_id,user_id,credentials_cipher,subscription_cipher,created_at) VALUES(?,?,?,?,?)`, item.proxy, item.old, credentials, token, at); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{
		`INSERT INTO balances(user_id,txb_minor,updated_at) VALUES('old-member-a',12345,'` + at + `')`,
		`INSERT INTO ledger_entries(id,user_id,delta_txb_minor,balance_after,kind,reference_id,created_at) VALUES('original-ledger','old-member-a',12345,12345,'original','original','` + at + `')`,
		`INSERT INTO xnode_configs(id,name,document_cipher,created_at,updated_at) VALUES('config-a','Configuration','unused','` + at + `','` + at + `')`,
		`INSERT INTO xnode_nodes(id,name,config_id,settings_cipher,agent_id,created_at,updated_at) VALUES('node-a','Node','config-a','unused','agent-a','` + at + `','` + at + `')`,
		`INSERT INTO xnode_periods(id,proxy_user_id,started_at,limit_bytes,strategy,used_bytes,status) VALUES('original-period',42,'` + at + `',100000,'MONTH',23,'open')`,
		`INSERT INTO xnode_period_nodes(period_id,node_id) VALUES('original-period','node-a')`,
		`UPDATE xnode_identities SET current_period_id='original-period' WHERE proxy_user_id=42`,
	} {
		if _, err = db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.EnqueueOutbox(ctx, "xnode_access_repair", `{"userId":"old-member-a","proxyUserId":"42"}`, now); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateTelegramIdentities(ctx, vault); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateTelegramIdentities(ctx, vault); err != nil {
		t.Fatal("repeat migration", err)
	}
	user, err := store.UserByID(ctx, "old-member-a")
	if err != nil || user.ID != "99101" {
		t.Fatalf("canonical user: %+v %v", user, err)
	}
	identity, err := store.XNodeIdentityByUser(ctx, user.ID)
	if err != nil || identity.ProxyUserID != 99101 || identity.ReplacesID != 42 {
		t.Fatalf("canonical node identity: %+v %v", identity, err)
	}
	plain, err := vault.Decrypt("xnode.identity.user.99101", identity.CredentialsCipher)
	if err != nil || plain != `{"uuid":"secret-0"}` {
		t.Fatalf("rotated credentials: %s %v", plain, err)
	}
	token, err := vault.Decrypt("xnode.subscription.user.99101", identity.SubscriptionCipher)
	if err != nil || token != strings.Repeat("0", 43) {
		t.Fatal("subscription link changed", err)
	}
	var usernameColumns, balance, delta int64
	if err = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('users') WHERE name='username'`).Scan(&usernameColumns); err != nil || usernameColumns != 0 {
		t.Fatal("assigned username retained", err)
	}
	if err = db.QueryRow(`SELECT txb_minor FROM balances WHERE user_id='99101'`).Scan(&balance); err != nil || balance != 12345 {
		t.Fatal("balance changed", err)
	}
	if err = db.QueryRow(`SELECT delta_txb_minor FROM ledger_entries WHERE user_id='99101'`).Scan(&delta); err != nil || delta != 12345 {
		t.Fatal("financial evidence changed", err)
	}
	var payload string
	if err = db.QueryRow(`SELECT payload FROM outbox_jobs WHERE kind='xnode_access_repair'`).Scan(&payload); err != nil || !strings.Contains(payload, `"userId":"99101"`) {
		t.Fatal("queued owner not migrated", payload, err)
	}
	report := wire.TrafficReport{ID: "agent-a:1", Samples: []wire.TrafficSample{{UserID: 42, AccountingPeriod: "original-period", From: now, Through: now.Add(time.Second), UploadBytes: 13, DownloadBytes: 17}}}
	for range 2 {
		if err = store.RecordXNodeTraffic(ctx, "node-a", "agent-a", 1, "original-digest", report); err != nil {
			t.Fatal("late traffic", err)
		}
	}
	period, err := store.XNodePeriod(ctx, "original-period")
	if err != nil || period.ProxyUserID != 99101 || period.UsedBytes != 53 {
		t.Fatalf("late bytes attributed through colliding numeric ID: %+v %v", period, err)
	}
	var violations int
	if err = db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatal("invalid references", err)
	}
}

func TestTelegramIdentityMigrationRollsBackOnUndecryptableCredentials(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	at := stamp(time.Now().UTC())
	if _, err = db.Exec(`INSERT INTO users(id,telegram_id,username,created_at,updated_at) VALUES('legacy-owner',99101,'reserved',?,?)`, at, at); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO xnode_identities(proxy_user_id,user_id,credentials_cipher,created_at) VALUES(42,'legacy-owner','invalid',?)`, at); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateTelegramIdentities(ctx, vault); err == nil {
		t.Fatal("invalid ciphertext migrated")
	}
	var id string
	if err = db.QueryRow(`SELECT id FROM users`).Scan(&id); err != nil || id != "legacy-owner" {
		t.Fatal("partial identity migration", err)
	}
	var enabled int
	if err = db.QueryRow(`PRAGMA foreign_keys`).Scan(&enabled); err != nil || enabled != 1 {
		t.Fatal("foreign keys left disabled", err)
	}
}
