package database

import (
	"context"
	"crypto/sha256"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Administrator revocation shares the immutable-material comparison with the
// member path, and commits only while its current administrator remains eligible.
func (s *Store) retryAttempt_RotateAdminXNodeSubscription(ctx context.Context, actor string, expected xnode.Identity, hash []byte, cipher string) (xnode.Identity, error) {
	if expected.ProxyUserID <= 0 || expected.ProxyUserID >= 1<<53 || len(hash) != sha256.Size || cipher == "" {
		return xnode.Identity{}, xnode.ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.Identity{}, err
	}
	defer tx.Rollback()
	if err = xnodeAdminActorTx(ctx, tx, actor); err != nil {
		return xnode.Identity{}, err
	}
	if owner := expected.MemberOwnerID(); owner != "" {
		if err = lockNativeMemberIdentityTx(ctx, tx, owner, 0, false); err != nil {
			return xnode.Identity{}, err
		}
	}
	current, err := rotateXNodeSubscriptionTx(ctx, tx, expected, hash, cipher)
	if err != nil {
		return xnode.Identity{}, err
	}
	if err = catalogAdminAuditTx(ctx, tx, actor, "xnode.subscription.rotate", "xnode_identity", strconv.FormatInt(current.ProxyUserID, 10), nil, time.Now().UTC()); err != nil {
		return xnode.Identity{}, err
	}
	if err = tx.Commit(); err != nil {
		return xnode.Identity{}, err
	}
	return current, nil
}
