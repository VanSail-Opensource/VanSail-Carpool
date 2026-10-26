package database

import (
	"context"
	"database/sql"
)

func emptyRenewalTx(ctx context.Context, tx *sql.Tx, purchase string) (bool, error) {
	var comboID string
	var customPrice sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT combo_id,reward_renewal_price_minor FROM purchases WHERE id=?`, purchase).Scan(&comboID, &customPrice); err != nil {
		return false, err
	}
	combo, err := comboByIDTx(ctx, tx, comboID, false)
	if err != nil {
		return false, err
	}
	combo, err = renewalComboTx(ctx, tx, purchase, combo)
	if err != nil {
		return false, err
	}
	addons, err := renewalAddonsTx(ctx, tx, purchase, nil)
	if customPrice.Valid {
		addons = nil
	}
	if err != nil || len(combo.IncludedSquads) > 0 || len(addons) > 0 {
		return false, err
	}
	owned, err := purchaseSquadsFrom(ctx, tx, purchase)
	if err != nil {
		return false, err
	}
	for _, id := range owned {
		var mode string
		e := tx.QueryRowContext(ctx, `SELECT billing_mode FROM squad_product_overrides WHERE group_id=?`, id).Scan(&mode)
		if e != nil && e != sql.ErrNoRows {
			return false, e
		}
		if mode == "payg" {
			return true, nil
		}
	}
	return false, nil
}

// automatic-renewal failure path (which deliberately forfeits its credit).
