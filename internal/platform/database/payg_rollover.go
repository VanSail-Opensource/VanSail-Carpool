package database

import (
	"context"
	"database/sql"
	"time"
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

// An empty next term settles the already reconciled rollover, never the
// automatic-renewal failure path (which deliberately forfeits its credit).
func (s *Store) settleEmptyRolloverTx(ctx context.Context, tx *sql.Tx, purchase string, now time.Time) error {
	value, err := scanRollover(tx.QueryRowContext(ctx, rolloverSelect+` WHERE purchase_id=?`, purchase))
	if err != nil {
		return err
	}
	var user string
	if err = tx.QueryRowContext(ctx, `SELECT user_id FROM purchases WHERE id=?`, purchase).Scan(&user); err != nil {
		return err
	}
	credit := calculatedRolloverCredit(value)
	if credit > 0 {
		balance, e := changeBalanceTx(ctx, tx, user, credit, now)
		if e != nil {
			return e
		}
		if _, e = insertLedgerTx(ctx, tx, user, credit, balance, "rollover_credit", purchase, "unused traffic rollover", now); e != nil {
			return e
		}
	}
	status := "zero"
	if credit > 0 {
		status = "credited"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE purchase_rollovers SET status=?,credited_txb_minor=?,completed_at=?,updated_at=? WHERE purchase_id=? AND status='calculated'`, status, credit, stamp(now), stamp(now), purchase); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE purchases SET status='expired',auto_renew_enabled=0,updated_at=? WHERE id=? AND status IN ('active','activating')`, stamp(now), purchase); err != nil {
		return err
	}
	if err = insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+user+`"}`, now, now); err != nil {
		return err
	}
	return s.insertExpirationNotificationTx(ctx, tx, purchase, userSyncGate(user), now)
}
