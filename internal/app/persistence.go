package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/backup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/datamigrate"
)

func openPersistence(ctx context.Context, cfg config.Config, logger *slog.Logger, versions []string) (*sql.DB, *postgres.Owner, error) {
	if cfg.Database.Driver != "postgres" {
		if _, err := backup.ApplyPendingRestore(ctx, cfg.DatabasePath, versions); err != nil {
			if _, statErr := os.Stat(cfg.DatabasePath); statErr != nil {
				return nil, nil, fmt.Errorf("apply pending database restore: %w", err)
			}
			logger.Error("pending database restore rolled back or was rejected", "error", err)
		}
	}
	db, err := database.OpenConfigured(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	var owner *postgres.Owner
	if database.IsPostgres(db) {
		owner, err = postgres.AcquireOwner(ctx, cfg.Database.URL)
		if err == nil {
			err = datamigrate.Activate(ctx, db, cfg.MasterKey)
		}
	} else {
		_, err = backup.RecordStartupRestore(ctx, db, cfg.DatabasePath)
	}
	if err != nil {
		owner.Close()
		db.Close()
		return nil, nil, err
	}
	return db, owner, nil
}

func maintenanceDatabasePath(cfg config.Config) string {
	if cfg.Database.Driver == "postgres" {
		return ""
	}
	return cfg.DatabasePath
}
