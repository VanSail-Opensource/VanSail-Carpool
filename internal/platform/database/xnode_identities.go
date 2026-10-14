package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const xnodeIdentitySelect = `SELECT p.proxy_user_id,COALESCE(p.user_id,''),COALESCE(u.username,p.service_name,''),COALESCE(p.service_name,''),p.credentials_cipher,p.subscription_hash,p.subscription_cipher,COALESCE(p.current_period_id,''),COALESCE(p.legacy_user_id,''),p.lifetime_base,p.access_disabled,p.created_at FROM xnode_identities p LEFT JOIN users u ON u.id=p.user_id`

func scanXNodeIdentity(row rowScanner) (xnode.Identity, error) {
	var item xnode.Identity
	var created string
	err := row.Scan(&item.ProxyUserID, &item.UserID, &item.Username, &item.ServiceName, &item.CredentialsCipher, &item.SubscriptionHash, &item.SubscriptionCipher, &item.CurrentPeriodID, &item.LegacyUserID, &item.LifetimeBase, &item.AccessDisabled, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return item, xnode.ErrNotFound
	}
	if err != nil {
		return item, err
	}
	item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	return item, err
}
func (s *Store) XNodeIdentities(ctx context.Context) ([]xnode.Identity, error) {
	rows, err := s.db.QueryContext(ctx, xnodeIdentitySelect+` ORDER BY p.proxy_user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []xnode.Identity{}
	for rows.Next() {
		item, err := scanXNodeIdentity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) XNodeIdentity(ctx context.Context, id int64) (xnode.Identity, error) {
	return scanXNodeIdentity(s.db.QueryRowContext(ctx, xnodeIdentitySelect+` WHERE p.proxy_user_id=?`, id))
}
func (s *Store) XNodeIdentityByUser(ctx context.Context, id string) (xnode.Identity, error) {
	return scanXNodeIdentity(s.db.QueryRowContext(ctx, xnodeIdentitySelect+` WHERE p.user_id=?`, id))
}
func (s *Store) XNodeIdentityBySubscription(ctx context.Context, hash []byte) (xnode.Identity, error) {
	return scanXNodeIdentity(s.db.QueryRowContext(ctx, xnodeIdentitySelect+` WHERE p.subscription_hash=?`, hash))
}
func (s *Store) SaveXNodeIdentity(ctx context.Context, item xnode.Identity) (xnode.Identity, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if item.ProxyUserID == 0 {
		// Reserve the lower ID space for linked legacy identities. Native/service
		// allocations cannot collide with new users still created by the bridge.
		var next int64
		if err := s.db.QueryRowContext(ctx, `SELECT MAX(COALESCE(MAX(proxy_user_id),0),1099511627775)+1 FROM xnode_identities`).Scan(&next); err != nil {
			return item, err
		}
		if next >= 1<<52 {
			return item, xnode.ErrConflict
		}
		result, err := s.db.ExecContext(ctx, `INSERT INTO xnode_identities(proxy_user_id,user_id,service_name,credentials_cipher,subscription_hash,subscription_cipher,current_period_id,legacy_user_id,lifetime_base,access_disabled,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, next, nullText(item.UserID), nullText(item.ServiceName), item.CredentialsCipher, item.SubscriptionHash, item.SubscriptionCipher, nullText(item.CurrentPeriodID), nullText(item.LegacyUserID), item.LifetimeBase, boolInt(item.AccessDisabled), identityCreatedAt(item))
		if err != nil {
			return item, err
		}
		item.ProxyUserID, err = result.LastInsertId()
		return item, err
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO xnode_identities(proxy_user_id,user_id,service_name,credentials_cipher,subscription_hash,subscription_cipher,current_period_id,legacy_user_id,lifetime_base,access_disabled,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(proxy_user_id) DO UPDATE SET credentials_cipher=excluded.credentials_cipher,subscription_hash=excluded.subscription_hash,subscription_cipher=excluded.subscription_cipher,current_period_id=excluded.current_period_id WHERE xnode_identities.user_id IS excluded.user_id AND xnode_identities.service_name IS excluded.service_name`, item.ProxyUserID, nullText(item.UserID), nullText(item.ServiceName), item.CredentialsCipher, item.SubscriptionHash, item.SubscriptionCipher, nullText(item.CurrentPeriodID), nullText(item.LegacyUserID), item.LifetimeBase, boolInt(item.AccessDisabled), identityCreatedAt(item))
	if err != nil {
		return item, err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		err = xnode.ErrConflict
	}
	return item, err
}

func nullText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func identityCreatedAt(item xnode.Identity) string {
	if !item.CreatedAt.IsZero() {
		return stamp(item.CreatedAt)
	}
	return stamp(time.Now().UTC())
}
