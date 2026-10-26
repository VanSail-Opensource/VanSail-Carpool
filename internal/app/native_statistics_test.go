package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNativeStatisticsNeverFallsBackToTheSourceProvider(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "statistics.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := database.NewStore(db)
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	settings := admin.NewSettingsService(store, vault)
	public, _ := url.Parse("https://panel.example")
	native := xnode.NewService(store, settings, vault, public, nil)
	master := xnode.NewMaster(native, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// The source adapter is deliberately unconfigured: consulting it would fail.
	provider := nativeStatisticsProvider{proxyProvider: proxyProvider{native: native, master: master, store: store}}
	now := time.Now().UTC()
	if _, err := provider.Digest(ctx, now.AddDate(0, 0, -7), now); err != nil {
		t.Fatal(err)
	}
	if value, err := provider.Traffic(ctx, now.AddDate(0, 0, -6), now); err != nil || len(value.Series) != 0 {
		t.Fatal(value, err)
	}
	if value, err := provider.Nodes(ctx); err != nil || len(value) != 0 {
		t.Fatal(value, err)
	}
	if value, err := provider.SquadNames(ctx); err != nil || len(value) != 0 {
		t.Fatal(value, err)
	}
	if _, err := provider.UsageSnapshot(ctx, "legacy-identity", now.Add(-time.Hour), now); !errors.Is(err, errors.New("account unavailable")) {
		t.Fatal("source accounting reader consulted", err)
	}
}
