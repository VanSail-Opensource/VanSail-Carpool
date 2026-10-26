package database

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// buyPAYGTx has no network work. Traffic reconciliation permits debt; an
// observed connection can purchase only from funds already in the wallet.
func buyPAYGTx(ctx context.Context, tx *sql.Tx, a *xnode.PAYGAccount, blocks int64, debt bool, now time.Time) error {
	if blocks <= 0 {
		return nil
	}
	public, unit, err := paygPricingTx(ctx, tx, a.UserID, a.GroupID, a.UnitPriceMinor, blocks, a.ProxyUserID, a.PurchasedBytes/xnode.PAYGGB, now)
	if err != nil {
		return err
	}
	a.UnitPriceMinor = public
	if unit <= 0 || blocks > math.MaxInt64/unit || blocks > (math.MaxInt64-a.PurchasedBytes)/xnode.PAYGGB || blocks > (math.MaxInt64-a.CreditBytes)/xnode.PAYGGB {
		return xnode.ErrInvalid
	}
	cost := blocks * unit
	var balance int64
	if debt {
		balance, err = adjustBalanceTx(ctx, tx, a.UserID, -cost, now)
	} else {
		balance, err = changeBalanceTx(ctx, tx, a.UserID, -cost, now)
	}
	if err != nil {
		return err
	}
	first := a.PurchasedBytes / xnode.PAYGGB
	ledger, err := insertLedgerTx(ctx, tx, a.UserID, -cost, balance, "payg_gb", fmt.Sprintf("%d:%d", a.ProxyUserID, first), "pay as you go", now)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_payg_charges(ledger_id,proxy_user_id,first_block,blocks,unit_price_minor) VALUES(?,?,?,?,?)`, ledger, a.ProxyUserID, first, blocks, unit); err != nil {
		return err
	}
	a.PurchasedBytes += blocks * xnode.PAYGGB
	a.CreditBytes += blocks * xnode.PAYGGB
	a.CreditStartedAt = now
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_payg_accounts SET purchased_bytes=?,unit_price_minor=?,credit_bytes=?,credit_started_at=? WHERE proxy_user_id=?`, a.PurchasedBytes, a.UnitPriceMinor, a.CreditBytes, stamp(now), a.ProxyUserID); err != nil {
		return err
	}
	return recordPAYGMinuteTx(ctx, tx, a.UserID, a.GroupID, now, 0, cost)
}

func recordPAYGMinuteTx(ctx context.Context, tx *sql.Tx, user, group string, at time.Time, traffic, spent int64) error {
	if at.Before(time.Now().UTC().Truncate(time.Minute).Add(-72 * time.Hour)) {
		return nil
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO xnode_payg_minutes(user_id,group_id,minute,traffic_bytes,spent_minor) VALUES(?,?,?,?,?) ON CONFLICT(user_id,group_id,minute) DO UPDATE SET traffic_bytes=xnode_payg_minutes.traffic_bytes+excluded.traffic_bytes,spent_minor=xnode_payg_minutes.spent_minor+excluded.spent_minor WHERE xnode_payg_minutes.traffic_bytes<=? AND xnode_payg_minutes.spent_minor<=?`, user, group, stamp(at.UTC().Truncate(time.Minute)), traffic, spent, math.MaxInt64-traffic, math.MaxInt64-spent)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return xnode.ErrInvalid
	}
	return nil
}

func (s *Store) XNodePAYGUsage(ctx context.Context, user string, from, to time.Time) ([]xnode.PAYGUsage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT group_id,SUM(traffic_bytes),SUM(spent_minor) FROM xnode_payg_minutes WHERE user_id=? AND minute>=? AND minute<? GROUP BY group_id`, user, stamp(from), stamp(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.PAYGUsage{}
	for rows.Next() {
		var u xnode.PAYGUsage
		if err = rows.Scan(&u.GroupID, &u.TrafficBytes, &u.SpentMinor); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
