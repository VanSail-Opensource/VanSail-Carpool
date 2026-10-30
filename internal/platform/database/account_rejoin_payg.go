package database

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// ReopenPurgedPAYG rotates credentials and opens an empty period atomically.
// Cumulative purchase/charge numbering remains untouched for historical evidence.
func (s *Store) retryAttempt_ReopenPurgedPAYG(ctx context.Context, identity xnode.Identity, period xnode.Period, user string, generation, price int64) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var allowed bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_payg_accounts a JOIN users u ON u.id=a.user_id JOIN xnode_identities i ON i.proxy_user_id=a.proxy_user_id JOIN xnode_periods p ON p.id=a.period_id WHERE a.proxy_user_id=? AND a.user_id=? AND u.account_generation=? AND u.account_status='active' AND i.credentials_cipher='' AND p.status='closed' AND a.credit_bytes=0)`, identity.ProxyUserID, user, generation).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_periods(id,proxy_user_id,started_at,limit_bytes,strategy,status) VALUES(?,?,?,0,'PAYG','open')`, period.ID, identity.ProxyUserID, stamp(period.StartedAt)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher=?,current_period_id=?,access_disabled=0 WHERE proxy_user_id=?`, identity.CredentialsCipher, period.ID, identity.ProxyUserID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_payg_accounts SET period_id=?,unit_price_minor=?,credit_bytes=0,credit_started_at=NULL WHERE proxy_user_id=?`, period.ID, price, identity.ProxyUserID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_period_nodes(period_id,node_id) SELECT ?,node_id FROM xnode_payg_accounts WHERE proxy_user_id=? ON CONFLICT(period_id,node_id) DO NOTHING`, period.ID, identity.ProxyUserID); err != nil {
		return err
	}
	return tx.Commit()
}
