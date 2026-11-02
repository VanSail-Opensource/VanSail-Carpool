package app

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

func openPersistence(ctx context.Context, cfg config.Config) (*sql.DB, *postgres.Owner, error) {
	db, err := database.OpenConfigured(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	owner, err := postgres.AcquireOwner(ctx, cfg.Database.URL)
	if err == nil {
		err = postgres.VerifySchema(ctx, db)
	}
	if err != nil {
		owner.Close()
		db.Close()
		return nil, nil, err
	}
	return db, owner, nil
}
