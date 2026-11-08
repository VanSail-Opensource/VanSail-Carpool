package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func lockLegacyHourlyStockTx(ctx context.Context, tx *sql.Tx, owner string, squads ...[]model.SquadProduct) error {
	groups := []string{}
	for _, products := range squads {
		for _, product := range products {
			groups = append(groups, product.GroupID)
		}
	}
	if err := lockSquadStockTx(ctx, tx, groups); err != nil {
		return err
	}
	return checkSquadStockTx(ctx, tx, uniqueSorted(groups), owner)
}
