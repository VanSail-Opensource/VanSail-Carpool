package database

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

// ConnectionIPOwnerAliases exposes retained upgrade evidence solely for opening
// an already-owned encrypted IP target. It never resolves a native identity.
func (s *Store) ConnectionIPOwnerAliases(ctx context.Context, owner string) ([]secret.ConnectionIPOwnerAlias, error) {
	return postgres.ConnectionIPOwnerAliases(ctx, s.db, owner)
}
