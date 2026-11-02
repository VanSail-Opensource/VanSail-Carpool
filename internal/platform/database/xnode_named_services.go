package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

// SaveXNodeNamedService commits encrypted terms, authorization and quota changes
// together. Existing usage and settled periods can never be rewritten by edits.
func (s *Store) retryAttempt_SaveXNodeNamedService(ctx context.Context, item xnode.Identity, access xnode.ServiceAccess) (xnode.Identity, error) {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return item, err
	}
	defer tx.Rollback()
	created := item.ProxyUserID == 0
	if created {
		query := `SELECT GREATEST(COALESCE(MAX(proxy_user_id),0),4503599627370495)+1 FROM xnode_identities`

		query = `SELECT nextval('xnode_service_proxy_id_seq')`

		if err = tx.QueryRowContext(ctx, query).Scan(&item.ProxyUserID); err != nil {
			return item, err
		}
		if item.ProxyUserID >= 1<<53 {
			return item, xnode.ErrConflict
		}
	}
	if item.CurrentPeriodID != "" {
		var status string
		if err = tx.QueryRowContext(ctx, `SELECT status FROM xnode_periods WHERE id=$1 AND proxy_user_id=$2`, item.CurrentPeriodID, item.ProxyUserID).Scan(&status); err != nil {
			return item, err
		}
		if status != "open" {
			return item, xnode.ErrConflict
		}
	}
	if created {
		_, err = tx.ExecContext(ctx, `INSERT INTO xnode_identities(proxy_user_id,service_name,credentials_cipher,subscription_hash,subscription_cipher,access_disabled,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, item.ProxyUserID, item.ServiceName, item.CredentialsCipher, item.SubscriptionHash, item.SubscriptionCipher, boolInt(access.Disabled), stamp(time.Now().UTC()))
	} else {
		result, e := tx.ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher=$1,access_disabled=$2 WHERE proxy_user_id=$3 AND user_id IS NULL AND service_name=$4 AND current_period_id IS NOT DISTINCT FROM $5`, item.CredentialsCipher, boolInt(access.Disabled), item.ProxyUserID, item.ServiceName, nullText(item.CurrentPeriodID))
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
		_, err = tx.ExecContext(ctx, `INSERT INTO xnode_periods(id,proxy_user_id,started_at,limit_bytes,strategy,last_reset_at,status) VALUES($1,$2,$3,$4,$5,$6,'open')`, item.CurrentPeriodID, item.ProxyUserID, stamp(access.StartedAt), access.LimitBytes, access.Strategy, stamp(access.StartedAt))
		if err != nil {
			return item, err
		}
		_, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET current_period_id=$1 WHERE proxy_user_id=$2`, item.CurrentPeriodID, item.ProxyUserID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE xnode_periods SET limit_bytes=$1,strategy=$2 WHERE id=$3`, access.LimitBytes, access.Strategy, item.CurrentPeriodID)
	}
	if err != nil {
		return item, err
	}
	if err = tx.Commit(); err != nil {
		return item, err
	}
	return s.XNodeIdentity(ctx, item.ProxyUserID)
}
