package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const paygSelect = `SELECT a.proxy_user_id,a.user_id,a.group_id,a.node_id,a.period_id,a.purchased_bytes,p.used_bytes,COALESCE(NULLIF(o.payg_price_minor,0),a.unit_price_minor),b.txb_minor FROM xnode_payg_accounts a JOIN xnode_periods p ON p.id=a.period_id JOIN balances b ON b.user_id=a.user_id LEFT JOIN squad_product_overrides o ON o.group_id=a.group_id`

func scanPAYG(row rowScanner) (xnode.PAYGAccount, error) {
	var a xnode.PAYGAccount
	err := row.Scan(&a.ProxyUserID, &a.UserID, &a.GroupID, &a.NodeID, &a.PeriodID, &a.PurchasedBytes, &a.UsedBytes, &a.UnitPriceMinor, &a.BalanceMinor)
	if errors.Is(err, sql.ErrNoRows) {
		err = xnode.ErrNotFound
	}
	return a, err
}

func (s *Store) XNodePAYGAccount(ctx context.Context, id int64) (xnode.PAYGAccount, error) {
	return scanPAYG(s.db.QueryRowContext(ctx, paygSelect+` WHERE a.proxy_user_id=?`, id))
}

func (s *Store) XNodePAYGAccounts(ctx context.Context, user, node string) ([]xnode.PAYGAccount, error) {
	query := paygSelect + ` WHERE 1=1`
	args := []any{}
	if user != "" {
		query += ` AND a.user_id=?`
		args = append(args, user)
	}
	if node != "" {
		query += ` AND a.node_id=?`
		args = append(args, node)
	}
	rows, err := s.db.QueryContext(ctx, query+` ORDER BY a.user_id,a.proxy_user_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.PAYGAccount{}
	for rows.Next() {
		a, e := scanPAYG(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) retryAttempt_SaveXNodePAYGAccount(ctx context.Context, a xnode.PAYGAccount) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_payg_accounts(proxy_user_id,user_id,group_id,node_id,period_id,unit_price_minor) VALUES(?,?,?,?,?,?) ON CONFLICT(user_id,group_id,node_id) DO NOTHING`, a.ProxyUserID, a.UserID, a.GroupID, a.NodeID, a.PeriodID, a.UnitPriceMinor)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_period_nodes(period_id,node_id) VALUES(?,?) ON CONFLICT(period_id,node_id) DO NOTHING`, a.PeriodID, a.NodeID); err != nil {
		return err
	}
	return tx.Commit()
}
