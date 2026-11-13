package database

import (
	"context"
	"crypto/sha256"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Token rotation never writes credential material, accounting pointers or
// readiness proof copied from an earlier read. A replaced snapshot must retry.
func (s *Store) retryAttempt_RotateXNodeSubscription(ctx context.Context, expected xnode.Identity, hash []byte, cipher string) (xnode.Identity, error) {
	if expected.ProxyUserID <= 0 || expected.ProxyUserID >= 1<<53 || len(hash) != sha256.Size || cipher == "" {
		return xnode.Identity{}, xnode.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Identity{}, err
	}
	defer tx.Rollback()
	if owner := expected.MemberOwnerID(); owner != "" {
		if err = lockNativeMemberIdentityTx(ctx, tx, owner, 0, false); err != nil {
			return xnode.Identity{}, err
		}
	}
	current, err := rotateXNodeSubscriptionTx(ctx, tx, expected, hash, cipher)
	if err != nil {
		return xnode.Identity{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Identity{}, err
	}
	return current, nil
}

// RotateXNodeMemberSubscription binds a member's request to its authenticated
// account generation as well as the exact native material it intends to revoke.
func (s *Store) retryAttempt_RotateXNodeMemberSubscription(ctx context.Context, expected xnode.Identity, generation int64, hash []byte, cipher string) (xnode.Identity, error) {
	if expected.UserID == "" || expected.ServiceName != "" || generation < 0 || expected.ProxyUserID <= 0 || expected.ProxyUserID >= 1<<53 || len(hash) != sha256.Size || cipher == "" {
		return xnode.Identity{}, xnode.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Identity{}, err
	}
	defer tx.Rollback()
	if err = lockNativeMemberIdentityTx(ctx, tx, expected.UserID, generation, true); err != nil {
		return xnode.Identity{}, err
	}
	current, err := rotateXNodeSubscriptionTx(ctx, tx, expected, hash, cipher)
	if err != nil {
		return xnode.Identity{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Identity{}, err
	}
	return current, nil
}

func rotateXNodeSubscriptionTx(ctx context.Context, tx *sql.Tx, expected xnode.Identity, hash []byte, cipher string) (xnode.Identity, error) {
	result, err := tx.ExecContext(ctx, `UPDATE xnode_identities SET subscription_hash=$1,subscription_cipher=$2
WHERE proxy_user_id=$3 AND user_id IS NOT DISTINCT FROM $4 AND service_name IS NOT DISTINCT FROM $5
 AND credentials_cipher=$6 AND subscription_hash IS NOT DISTINCT FROM $7 AND subscription_cipher=$8`,
		hash, cipher, expected.ProxyUserID, nullText(expected.UserID), nullText(expected.ServiceName),
		expected.CredentialsCipher, expected.SubscriptionHash, expected.SubscriptionCipher)
	if err != nil {
		return xnode.Identity{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return xnode.Identity{}, err
	}
	if affected != 1 {
		return xnode.Identity{}, xnode.ErrConflict
	}
	return scanXNodeIdentity(tx.QueryRowContext(ctx, xnodeIdentitySelect+` WHERE p.proxy_user_id=$1`, expected.ProxyUserID))
}
