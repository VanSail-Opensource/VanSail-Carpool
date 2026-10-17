package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

// SaveXNodeNamedService commits encrypted terms, authorization and quota changes
// together. Existing usage and settled periods can never be rewritten by edits.
func (s *Store) SaveXNodeNamedService(ctx context.Context, item xnode.Identity, access xnode.ServiceAccess) (xnode.Identity, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return item, err
	}
	defer tx.Rollback()
	created := item.ProxyUserID == 0
	if created {
		if err = tx.QueryRowContext(ctx, `SELECT MAX(COALESCE(MAX(proxy_user_id),0),1099511627775)+1 FROM xnode_identities`).Scan(&item.ProxyUserID); err != nil {
			return item, err
		}
		if item.ProxyUserID >= 1<<52 {
			return item, xnode.ErrConflict
		}
	}
	if item.CurrentPeriodID != "" {
		var status string
		if err = tx.QueryRowContext(ctx, `SELECT status FROM xnode_periods WHERE id=? AND proxy_user_id=?`, item.CurrentPeriodID, item.ProxyUserID).Scan(&status); err != nil {
			return item, err
		}
		if status != "open" {
			return item, xnode.ErrConflict
		}
	}
	if created {
		_, err = tx.ExecContext(ctx, `INSERT INTO xnode_identities(proxy_user_id,service_name,credentials_cipher,subscription_hash,subscription_cipher,access_disabled,created_at) VALUES(?,?,?,?,?,?,?)`, item.ProxyUserID, item.ServiceName, item.CredentialsCipher, item.SubscriptionHash, item.SubscriptionCipher, boolInt(access.Disabled), stamp(time.Now().UTC()))
	} else {
		result, e := tx.ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher=?,access_disabled=? WHERE proxy_user_id=? AND user_id IS NULL AND service_name=? AND current_period_id IS ?`, item.CredentialsCipher, boolInt(access.Disabled), item.ProxyUserID, item.ServiceName, nullText(item.CurrentPeriodID))
		if e != nil {
			return item, e
		}
		count, e := result.RowsAffected()
		if e != nil {
			return item, e
		}
		if count != 1 {
			return item, xnode.ErrConflict
		}
	}
	if err != nil {
		return item, err
	}
	if item.CurrentPeriodID == "" {
		item.CurrentPeriodID, err = ids.New()
		if err != nil {
			return item, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO xnode_periods(id,proxy_user_id,started_at,limit_bytes,strategy,last_reset_at,status) VALUES(?,?,?,?,?,?,'open')`, item.CurrentPeriodID, item.ProxyUserID, stamp(access.StartedAt), access.LimitBytes, access.Strategy, stamp(access.StartedAt))
		if err != nil {
			return item, err
		}
		_, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET current_period_id=? WHERE proxy_user_id=?`, item.CurrentPeriodID, item.ProxyUserID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE xnode_periods SET limit_bytes=?,strategy=? WHERE id=?`, access.LimitBytes, access.Strategy, item.CurrentPeriodID)
	}
	if err != nil {
		return item, err
	}
	if err = tx.Commit(); err != nil {
		return item, err
	}
	return s.XNodeIdentity(ctx, item.ProxyUserID)
}
