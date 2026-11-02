package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

// OpenConfigured opens the authoritative PostgreSQL database without a fallback.
func OpenConfigured(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	if cfg.Database.Driver != "postgres" {
		return nil, fmt.Errorf("PostgreSQL is required")
	}
	return postgres.Open(ctx, cfg.Database)
}
