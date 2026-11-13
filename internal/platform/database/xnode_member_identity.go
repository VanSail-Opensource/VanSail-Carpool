package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// SaveXNodeMemberIdentity provisions only the captured owner generation and
// credential material. Period pointers and unrelated current state are retained.
func (s *Store) retryAttempt_SaveXNodeMemberIdentity(ctx context.Context, expected, next xnode.Identity, generation int64) (xnode.Identity, error) {
	if generation < 0 || expected.UserID == "" || expected.UserID != next.UserID || expected.ServiceName != "" || next.ServiceName != "" ||
		expected.ProxyUserID != next.ProxyUserID || next.CredentialsCipher == "" || next.SubscriptionCipher == "" || len(next.SubscriptionHash) != sha256.Size {
		return xnode.Identity{}, xnode.ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Identity{}, err
	}
	defer tx.Rollback()
	if err = lockNativeMemberIdentityTx(ctx, tx, expected.UserID, generation, true); err != nil {
		return xnode.Identity{}, err
	}
	var telegramID int64
	var eligible bool
	if err = tx.QueryRowContext(ctx, `SELECT telegram_id,bot_blocked=0 FROM users WHERE id=$1`, expected.UserID).Scan(&telegramID, &eligible); err != nil {
		return xnode.Identity{}, err
	}
	if !eligible || telegramID <= 0 || telegramID >= 1<<53 {
		return xnode.Identity{}, ErrAccountUnavailable
	}
	current, err := scanXNodeIdentity(tx.QueryRowContext(ctx, xnodeIdentitySelect+` WHERE p.user_id=$1 FOR UPDATE OF p`, expected.UserID))
	if errors.Is(err, xnode.ErrNotFound) {
		if expected.ProxyUserID != 0 {
			return xnode.Identity{}, xnode.ErrConflict
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO xnode_identities(proxy_user_id,user_id,credentials_cipher,subscription_hash,subscription_cipher,access_disabled,created_at)
			VALUES($1,$2,$3,$4,$5,CASE WHEN EXISTS(SELECT 1 FROM admin_temporary_bans WHERE user_id=$2 AND restored_at IS NULL) THEN 1 ELSE 0 END,$6)`, telegramID, next.UserID, next.CredentialsCipher, next.SubscriptionHash, next.SubscriptionCipher, identityCreatedAt(next))
	} else if err == nil {
		if expected.ProxyUserID != current.ProxyUserID || current.ServiceName != "" {
			return xnode.Identity{}, xnode.ErrConflict
		}
		if !sameProvisioningMaterial(current, expected) {
			if sameProvisioningMaterial(current, next) {
				return current, nil
			}
			return xnode.Identity{}, xnode.ErrConflict
		}
		// Only rejoining a previously cleared principal may lift its purge
		// restriction. A normal repair cannot erase an administrator's ban.
		_, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher=$1,subscription_hash=$2,subscription_cipher=$3,
			first_synced_at=CASE WHEN credentials_cipher IS DISTINCT FROM $1 THEN NULL ELSE first_synced_at END,
			access_disabled=CASE WHEN EXISTS(SELECT 1 FROM admin_temporary_bans WHERE user_id=$6 AND restored_at IS NULL) THEN 1
			WHEN credentials_cipher='' AND $4::bigint>0 THEN 0 ELSE access_disabled END WHERE proxy_user_id=$5`,
			next.CredentialsCipher, next.SubscriptionHash, next.SubscriptionCipher, generation, current.ProxyUserID, current.UserID)
	}
	if err != nil {
		return xnode.Identity{}, err
	}
	result, err := scanXNodeIdentity(tx.QueryRowContext(ctx, xnodeIdentitySelect+` WHERE p.user_id=$1`, expected.UserID))
	if err != nil {
		return xnode.Identity{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Identity{}, err
	}
	return result, nil
}

func sameProvisioningMaterial(left, right xnode.Identity) bool {
	return left.CredentialsCipher == right.CredentialsCipher && left.SubscriptionCipher == right.SubscriptionCipher && bytes.Equal(left.SubscriptionHash, right.SubscriptionHash)
}
