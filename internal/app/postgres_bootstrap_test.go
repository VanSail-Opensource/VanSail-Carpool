package app

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/jackc/pgx/v5"
)

func TestPostgresApplicationBootstrap(t *testing.T) {
	url := os.Getenv("POSTGRES_BOOTSTRAP_ADMIN_URL")
	if url == "" {
		t.Skip("hosted migrated PostgreSQL fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	parsed, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Database = "postgres"
	admin, err := pgx.ConnectConfig(ctx, parsed)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err = admin.Exec(ctx, `CREATE DATABASE vansail_application_rehearsal TEMPLATE vansail OWNER vansail_migration`); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), `DROP DATABASE vansail_application_rehearsal WITH (FORCE)`)
	applicationURL, err := neturl.Parse(os.Getenv("POSTGRES_BOOTSTRAP_APP_URL"))
	if err != nil {
		t.Fatal(err)
	}
	applicationURL.Path = "/vansail_application_rehearsal"
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", applicationURL.String())
	t.Setenv("DATABASE_URL_FILE", "")
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv("CONFIG_MASTER_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	t.Setenv("ADMIN_TELEGRAM_ID", "912001")
	t.Setenv("TELEGRAM_BOT_TOKEN", "123456:hosted-rehearsal-only")
	t.Setenv("PUBLIC_BASE_URL", "https://rehearsal.invalid")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	app, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if app != nil {
			_ = app.Close()
		}
	}()
	request := httptest.NewRequest("GET", "http://rehearsal.invalid/healthz", nil)
	response := httptest.NewRecorder()
	app.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("health: %d %s", response.Code, response.Body.String())
	}
	if second, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		second.Close()
		t.Fatal("second application acquired scheduler ownership")
	}
	var phase string
	var role string
	if err = app.store.DB().QueryRowContext(ctx, `SELECT current_user`).Scan(&role); err != nil || role != "vansail_app" {
		t.Fatal("application did not use restricted role", err)
	}
	if err = app.store.DB().QueryRowContext(ctx, `SELECT phase FROM vansail_migration_state WHERE singleton=1`).Scan(&phase); err != nil || phase != "active" {
		t.Fatal("verified activation failed", err)
	}
	// Simulate a referral account created by an older release after activation.
	// Its stable owner ID and financial references must survive a restart.
	if _, err = app.store.DB().ExecContext(ctx, `INSERT INTO users(id,telegram_id,created_at,updated_at) VALUES('legacy-referral-owner',912099,?,?)`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err = app.store.DB().ExecContext(ctx, `INSERT INTO balances(user_id,txb_minor,updated_at) VALUES('legacy-referral-owner',123,?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err = app.Close(); err != nil {
		t.Fatal(err)
	}
	app = nil
	app, err = New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal("restart with post-activation referral", err)
	}
	var balance int64
	if err = app.store.DB().QueryRowContext(ctx, `SELECT txb_minor FROM balances WHERE user_id='legacy-referral-owner'`).Scan(&balance); err != nil || balance != 123 {
		t.Fatalf("legacy balance after restart: %d, %v", balance, err)
	}
}
