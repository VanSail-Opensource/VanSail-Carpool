package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// A core-change prize changes the base of a custom combo without revoking its
// awarded terms. Ordinary combos continue to adopt the new core's defaults.
func applyDrawCoreComboTx(ctx context.Context, tx *sql.Tx, purchaseID, comboID string, now time.Time) error {
	combo, err := comboByIDTx(ctx, tx, comboID, true)
	if err != nil {
		return err
	}
	var customPrice sql.NullInt64
	var traffic int64
	var resetStrategy string
	if err := tx.QueryRowContext(ctx, `SELECT p.reward_renewal_price_minor,
		COALESCE(p.entitlement_traffic_limit_bytes,c.traffic_limit_bytes),COALESCE(p.entitlement_reset_strategy,c.reset_strategy)
		FROM purchases p JOIN combos c ON c.id=p.combo_id WHERE p.id=$1`, purchaseID).Scan(&customPrice, &traffic, &resetStrategy); err != nil {
		return fmt.Errorf("load core-change reward terms: %w", err)
	}
	if customPrice.Valid {
		// Materialize legacy inherited cadence before changing its base combo.
		_, err = tx.ExecContext(ctx, `UPDATE purchases SET combo_id=$1,entitlement_reset_strategy=$2,updated_at=$3 WHERE id=$4`, comboID, resetStrategy, stamp(now), purchaseID)
	} else {
		traffic = combo.TrafficLimitBytes
		_, err = tx.ExecContext(ctx, `UPDATE purchases SET combo_id=$1,entitlement_traffic_limit_bytes=NULL,
			entitlement_reset_strategy=NULL,entitlement_squad_uuids=NULL,entitlement_addon_squad_uuids=NULL,
			reward_traffic_renewal=1,reward_renewal_traffic_limit_bytes=NULL,updated_at=$2 WHERE id=$3`, comboID, stamp(now), purchaseID)
	}
	if err != nil {
		return fmt.Errorf("apply core-change reward: %w", err)
	}
	return nil
}
