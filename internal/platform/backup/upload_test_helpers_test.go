package backup

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func newUploadTestService(t *testing.T) (*Service, *database.Store, context.Context, []string) {
	t.Helper()
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "tx-carpool.db")
	db, err := database.Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("database.Open(): %v", err)
	}
	store := database.NewStore(db)
	t.Cleanup(func() { _ = db.Close() })
	migrations, err := database.MigrationVersions()
	if err != nil {
		t.Fatalf("MigrationVersions(): %v", err)
	}
	service := NewService(db, store, filepath.Join(t.TempDir(), "backups"), 24*time.Hour)
	return service, store, ctx, migrations
}
