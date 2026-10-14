package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// renewalAddonsTx retains purchased squad identities and prices them from the
// current sparse overrides. Visibility controls new sales, not owned renewals;
// an absent override means the catalog's default zero price. The catalog service
// must still validate every retained identity against the queued live provider.
func renewalAddonsTx(ctx context.Context, tx *sql.Tx, purchaseID string, excludedAddonIDs []string) ([]model.SquadProduct, error) {
	excluded := make(map[string]struct{}, len(excludedAddonIDs))
	for _, id := range excludedAddonIDs {
		if id = strings.TrimSpace(id); id != "" {
			excluded[id] = struct{}{}
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT a.remna_squad_uuid,COALESCE(o.price_txb_minor,0)
		FROM purchase_addons a LEFT JOIN squad_product_overrides o ON o.remna_squad_uuid=a.remna_squad_uuid
		WHERE a.purchase_id=? ORDER BY a.remna_squad_uuid`, purchaseID)
	if err != nil {
		return nil, fmt.Errorf("load current renewal add-ons: %w", err)
	}
	defer func() { _ = rows.Close() }()
	addons := make([]model.SquadProduct, 0)
	for rows.Next() {
		var addon model.SquadProduct
		if err := rows.Scan(&addon.RemnaSquadUUID, &addon.PriceTXBMinor); err != nil {
			return nil, fmt.Errorf("scan current renewal add-on: %w", err)
		}
		addon.ID = addon.RemnaSquadUUID
		addon.Price = model.TXBMoney(addon.PriceTXBMinor)
		if _, skip := excluded[addon.RemnaSquadUUID]; skip {
			continue
		}
		addons = append(addons, addon)
	}
	return addons, rows.Err()
}
