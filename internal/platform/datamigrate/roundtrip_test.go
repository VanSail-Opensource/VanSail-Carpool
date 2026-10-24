package datamigrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func TestPostgresExactRoundTrip(t *testing.T) {
	url := os.Getenv("POSTGRES_MIGRATION_TEST_URL")
	if url == "" {
		t.Skip("hosted PostgreSQL recovery environment required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	path := filepath.Join(t.TempDir(), "source #%!.db")
	db, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	vault, err := secret.NewVault(key)
	if err != nil {
		t.Fatal(err)
	}
	store := database.NewStore(db)
	if err = store.MigrateTelegramIdentities(ctx, vault); err != nil {
		t.Fatal(err)
	}
	user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 912001, FirstName: "Migration"}, false)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 24, 0, 0, 0, 123456789, time.UTC)
	if _, err = store.AdjustBalance(ctx, user.ID, 123456789, "migration-reference", "exact", now); err != nil {
		t.Fatal(err)
	}
	cipher, err := vault.Encrypt("migration.fixture", "exact secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at) VALUES(?,?,1,?)`, "migration.fixture", cipher, now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at,created_at) VALUES(?,?,?,?)`, []byte{0, 1, 2, 255}, user.ID, now.Add(time.Hour).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err = store.EnqueueOutbox(ctx, "migration_pending", `{"userId":"912001","nested":{"exact":1}}`, now); err != nil {
		t.Fatal(err)
	}
	seedAccounting(t, ctx, store, vault, user.ID, now)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(t.TempDir(), "snapshot.db")
	exported, err := Export(ctx, path, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Export(ctx, path, snapshot); err == nil {
		t.Fatal("snapshot overwrite accepted")
	}
	source, err := OpenSource(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	cfg := config.DatabaseConfig{Driver: "postgres", URL: url, MaxOpenConns: 4, MaxIdleConns: 2, ConnectTimeout: 10 * time.Second, StatementTimeout: time.Minute, LockTimeout: 5 * time.Second}
	target, err := postgres.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	assertImportRollback(t, ctx, snapshot, url, target)
	imported, err := Import(ctx, source, url)
	if err != nil {
		t.Fatal(err)
	}
	if imported.Hash != exported.Hash {
		t.Fatal("import evidence differs")
	}
	verified, err := Verify(ctx, source, target, key)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Hash != exported.Hash {
		t.Fatal("verification changed evidence")
	}
	if _, err = CutoverCheck(ctx, target); err != nil {
		t.Fatal(err)
	}
	if _, err = Import(ctx, source, url); err == nil {
		t.Fatal("populated target accepted")
	}
	wrong := make([]byte, 32)
	wrong[0] = 1
	if _, err = Verify(ctx, source, target, wrong); err == nil {
		t.Fatal("wrong recovery key accepted")
	}
	if _, err = target.ExecContext(ctx, `UPDATE balances SET txb_minor=txb_minor+1 WHERE user_id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = CutoverCheck(ctx, target); err == nil {
		t.Fatal("changed target passed cutover")
	}
	if _, err = target.ExecContext(ctx, `UPDATE balances SET txb_minor=txb_minor-1 WHERE user_id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
}
